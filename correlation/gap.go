package correlation

import (
	"math"
	"time"

	"github.com/theapemachine/datura"
	"gonum.org/v1/gonum/stat"
)

const gapPayloadHeader = 2

/*
Gap compares asynchronous Hayashi-Yoshida coupling to synchronous Pearson correlation.

Payload layout: syncCount, asyncPairCount, syncLeft..., syncRight..., asyncLeft..., asyncRight...
Async segments encode time-value pairs. maxInterval may be set via config.maxIntervalSeconds.
*/
type Gap struct {
	artifact    *datura.Artifact
	weights     []float64
	maxInterval time.Duration
}

/*
NewGap creates a dual-correlation gap stage.
*/
func NewGap(weights []float64, maxInterval time.Duration) *Gap {
	return &Gap{
		artifact:    datura.Acquire("correlate-gap", datura.APPJSON).RetainStageAttributes(),
		weights:     weights,
		maxInterval: maxInterval,
	}
}

func (gap *Gap) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](gap.artifact, "output") == nil

	gap.artifact.Clear("sample")
	gap.artifact.Clear("batch")

	n, err := gap.artifact.Write(p)

	if bootstrap {
		gap.artifact.Clear("output")
	}

	return n, err
}

func (gap *Gap) Read(p []byte) (int, error) {
	batch := gapBatch(gap.artifact)
	pearsonValue := gapPearson(batch, gap.weights)
	hayashiValue := gapHayashi(batch, gap.maxIntervalFromArtifact())
	divergence := hayashiValue - pearsonValue

	if math.IsNaN(divergence) || math.IsInf(divergence, 0) {
		divergence = 0
	}

	gap.artifact.Poke(datura.Map[float64]{
		"value":   divergence,
		"pearson": pearsonValue,
		"hayashi": hayashiValue,
		"gap":     divergence,
	}, "output")

	return gap.artifact.Read(p)
}

func (gap *Gap) Close() error {
	return nil
}

func (gap *Gap) maxIntervalFromArtifact() time.Duration {
	if gap.maxInterval > 0 {
		return gap.maxInterval
	}

	seconds := datura.Peek[float64](gap.artifact, "config", "maxIntervalSeconds")

	if seconds <= 0 {
		return 0
	}

	return time.Duration(seconds * float64(time.Second))
}

func gapBatch(artifact *datura.Artifact) []float64 {
	values := datura.Peek[[]float64](artifact, "batch")

	if len(values) > 0 {
		return values
	}

	payload, ok := artifact.PayloadQuiet()

	if !ok || len(payload) == 0 || len(payload)%8 != 0 {
		return nil
	}

	samples := make([]float64, len(payload)/8)

	for index := range samples {
		offset := index * 8
		value := math.Float64frombits(
			uint64(payload[offset])<<56 |
				uint64(payload[offset+1])<<48 |
				uint64(payload[offset+2])<<40 |
				uint64(payload[offset+3])<<32 |
				uint64(payload[offset+4])<<24 |
				uint64(payload[offset+5])<<16 |
				uint64(payload[offset+6])<<8 |
				uint64(payload[offset+7]),
		)

		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil
		}

		samples[index] = value
	}

	return samples
}

func gapSegments(batch []float64) (syncLeft, syncRight, asyncLeft, asyncRight []float64, ok bool) {
	if len(batch) < gapPayloadHeader {
		return nil, nil, nil, nil, false
	}

	syncCount := int(batch[0])
	asyncPairCount := int(batch[1])
	offset := gapPayloadHeader

	if syncCount < 2 || asyncPairCount < 2 {
		return nil, nil, nil, nil, false
	}

	asyncScalarCount := asyncPairCount * 2

	if offset+syncCount+syncCount+asyncScalarCount+asyncScalarCount > len(batch) {
		return nil, nil, nil, nil, false
	}

	syncLeft = batch[offset : offset+syncCount]
	offset += syncCount
	syncRight = batch[offset : offset+syncCount]
	offset += syncCount
	asyncLeft = batch[offset : offset+asyncScalarCount]
	offset += asyncScalarCount
	asyncRight = batch[offset : offset+asyncScalarCount]

	return syncLeft, syncRight, asyncLeft, asyncRight, true
}

func gapPearson(batch []float64, weights []float64) float64 {
	syncLeft, syncRight, _, _, ok := gapSegments(batch)

	if !ok || len(syncLeft) < 2 || len(syncLeft) != len(syncRight) {
		return 0
	}

	weightsOK := len(weights) == 0 || len(weights) == len(syncLeft)

	if !weightsOK {
		return 0
	}

	sampleWeights := weights

	if len(sampleWeights) == 0 {
		sampleWeights = nil
	}

	correlationValue := stat.Correlation(syncLeft, syncRight, sampleWeights)

	if math.IsNaN(correlationValue) || math.IsInf(correlationValue, 0) {
		return 0
	}

	return correlationValue
}

func gapHayashi(batch []float64, maxInterval time.Duration) float64 {
	_, _, asyncLeft, asyncRight, ok := gapSegments(batch)

	if !ok {
		return 0
	}

	left, leftOK := samplesFromScalars(asyncLeft)
	right, rightOK := samplesFromScalars(asyncRight)

	if !leftOK || !rightOK {
		return 0
	}

	correlationValue, hayashiOK := hayashiYoshidaCorrelation(left, right, maxInterval)

	if !hayashiOK {
		return 0
	}

	return correlationValue
}

func EncodeGapBatch(
	syncLeft, syncRight, asyncLeft, asyncRight []float64,
) []float64 {
	if len(syncLeft) != len(syncRight) {
		return nil
	}

	if len(asyncLeft) == 0 || len(asyncLeft)%2 != 0 || len(asyncLeft) != len(asyncRight) {
		return nil
	}

	batch := make(
		[]float64,
		0,
		gapPayloadHeader+len(syncLeft)+len(syncRight)+len(asyncLeft)+len(asyncRight),
	)
	batch = append(batch, float64(len(syncLeft)), float64(len(asyncLeft)/2))
	batch = append(batch, syncLeft...)
	batch = append(batch, syncRight...)
	batch = append(batch, asyncLeft...)
	batch = append(batch, asyncRight...)

	return batch
}

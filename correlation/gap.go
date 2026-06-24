package correlation

import (
	"math"
	"time"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

const gapPayloadHeader = 2

/*
Gap compares asynchronous Hayashi-Yoshida coupling to synchronous Pearson correlation.

Payload layout: syncCount, asyncPairCount, syncLeft..., syncRight..., asyncLeft..., asyncRight...
Weights and maxIntervalSeconds may be set on config.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Gap struct {
	artifact *datura.Artifact
}

/*
NewGap creates a dual-correlation gap stage wired from config attributes on the artifact.
*/
func NewGap(artifact *datura.Artifact) *Gap {
	return &Gap{
		artifact: artifact,
	}
}

func (gap *Gap) Read(p []byte) (int, error) {
	state := datura.Acquire("gap-state", datura.APPJSON)

	if _, err := state.Write(gap.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	state.Inspect("correlation", "gap", "Read()", "p")

	batch := gapBatch(state)

	if len(batch) < gapPayloadHeader {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "unable to compute correlation gap",
			GapError(GapErrorRequireBatch),
		))
	}

	weights := datura.Peek[[]float64](gap.artifact, "config", "weights")
	pearsonValue, pearsonOK := gapPearson(batch, weights)

	if !pearsonOK {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "unable to compute correlation gap",
			GapError(GapErrorRequireSyncSegments),
		))
	}

	hayashiValue, hayashiOK := gapHayashi(batch, gap.maxIntervalFromArtifact())

	if !hayashiOK {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "unable to compute correlation gap",
			GapError(GapErrorRequireAsyncSegments),
		))
	}

	divergence := hayashiValue - pearsonValue

	if math.IsNaN(divergence) || math.IsInf(divergence, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "unable to compute correlation gap",
			GapError(GapErrorNonFiniteGap),
		))
	}

	state.MergeOutput("value", divergence)
	state.MergeOutput("pearson", pearsonValue)
	state.MergeOutput("hayashi", hayashiValue)
	state.MergeOutput("gap", divergence)
	state.Poke("output", "root")
	state.Poke([]string{"value", "pearson", "hayashi", "gap"}, "inputs")
	return state.Read(p)
}

func (gap *Gap) Write(p []byte) (int, error) {
	gap.artifact.WithPayload(p)
	return len(p), nil
}

func (gap *Gap) Close() error {
	return nil
}

func (gap *Gap) maxIntervalFromArtifact() time.Duration {
	seconds := datura.Peek[float64](gap.artifact, "config", "maxIntervalSeconds")

	if seconds <= 0 {
		return 0
	}

	return time.Duration(seconds * float64(time.Second))
}

func gapBatch(state *datura.Artifact) []float64 {
	values := datura.Peek[[]float64](state, "batch")

	if len(values) > 0 {
		return values
	}

	values = datura.Peek[[]float64](state, "features")

	if len(values) > 0 {
		return values
	}

	payload := state.DecryptPayload()

	if len(payload) == 0 || len(payload)%8 != 0 {
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

func gapPearson(batch []float64, weights []float64) (float64, bool) {
	syncLeft, syncRight, _, _, ok := gapSegments(batch)

	if !ok || len(syncLeft) < 2 || len(syncLeft) != len(syncRight) {
		return 0, false
	}

	weightsOK := len(weights) == 0 || len(weights) == len(syncLeft)

	if !weightsOK {
		return 0, false
	}

	sampleWeights := weights

	if len(sampleWeights) == 0 {
		sampleWeights = nil
	}

	correlationValue := stat.Correlation(syncLeft, syncRight, sampleWeights)

	if math.IsNaN(correlationValue) || math.IsInf(correlationValue, 0) {
		return 0, false
	}

	return correlationValue, true
}

func gapHayashi(batch []float64, maxInterval time.Duration) (float64, bool) {
	_, _, asyncLeft, asyncRight, ok := gapSegments(batch)

	if !ok {
		return 0, false
	}

	left, leftOK := samplesFromScalars(asyncLeft)
	right, rightOK := samplesFromScalars(asyncRight)

	if !leftOK || !rightOK {
		return 0, false
	}

	return hayashiYoshidaCorrelation(left, right, maxInterval)
}

type GapErrorType string

const (
	GapErrorRequireBatch         GapErrorType = "require gap batch payload"
	GapErrorRequireSyncSegments  GapErrorType = "require valid synchronous segments"
	GapErrorRequireAsyncSegments GapErrorType = "require valid asynchronous segments"
	GapErrorNonFiniteGap         GapErrorType = "require finite gap"
)

type GapError string

func (gapError GapError) Error() string {
	return string(gapError)
}

package correlation

import (
	"encoding/binary"
	"math"

	"github.com/theapemachine/datura"
)

func float64Batch(artifact *datura.Artifact) []float64 {
	if artifact == nil {
		return nil
	}

	payload, err := artifact.Payload()

	if err != nil || len(payload) < 8 || len(payload)%8 != 0 {
		return nil
	}

	return float64sFromPayload(payload)
}

func float64sFromPayload(payload []byte) []float64 {
	count := len(payload) / 8
	values := make([]float64, count)

	for index := range count {
		offset := index * 8
		value := math.Float64frombits(binary.BigEndian.Uint64(payload[offset : offset+8]))

		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil
		}

		values[index] = value
	}

	return values
}

func putFloat64Payload(artifact **datura.Artifact, name string, value float64) {
	*artifact = datura.Acquire(name, datura.Artifact_Type_json)
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, math.Float64bits(value))
	_ = (*artifact).SetPayload(payload)
}

func artifactBytes(artifact *datura.Artifact) ([]byte, bool) {
	buf, err := artifact.Message().Marshal()

	if err != nil {
		return nil, false
	}

	return buf, true
}

func seriesOutput(series *IntervalSeries) float64 {
	if series == nil {
		return 0
	}

	return series.LastReturnMagnitude()
}

func parseEpochLevel(artifact *datura.Artifact) (int64, float64, bool) {
	values := float64Batch(artifact)

	if len(values) < 2 {
		return 0, 0, false
	}

	level := values[1]

	if level <= 0 {
		return 0, 0, false
	}

	return int64(values[0]), level, true
}

func weightSamples(weights []float64) []float64 {
	if len(weights) == 0 {
		return nil
	}

	return weights
}

func weightSamplesFor(weights []float64, count int) ([]float64, bool) {
	if len(weights) == 0 {
		return nil, true
	}

	if len(weights) != count {
		return nil, false
	}

	for _, weight := range weights {
		if math.IsNaN(weight) || math.IsInf(weight, 0) || weight < 0 {
			return nil, false
		}
	}

	return weights, true
}

package statistic

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
		values[index] = math.Float64frombits(binary.BigEndian.Uint64(payload[offset : offset+8]))
	}

	return values
}

func boundaryFloat64(artifact *datura.Artifact) (float64, bool) {
	payload, err := artifact.Payload()

	if err != nil || len(payload) != 8 {
		return 0, false
	}

	return math.Float64frombits(binary.BigEndian.Uint64(payload)), true
}

func pairFloat64s(artifact *datura.Artifact) (float64, float64, bool) {
	values := float64Batch(artifact)

	if len(values) >= 2 {
		return values[0], values[1], true
	}

	return 0, 0, false
}

func putFloat64Payload(artifact **datura.Artifact, name string, value float64) {
	*artifact = datura.Acquire(name, datura.Artifact_Type_json)
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, math.Float64bits(value))
	_ = (*artifact).SetPayload(payload)
}

func putFloat64sPayload(artifact *datura.Artifact, values ...float64) {
	out := make([]byte, 8*len(values))

	for index, value := range values {
		offset := index * 8
		binary.BigEndian.PutUint64(out[offset:offset+8], math.Float64bits(value))
	}

	_ = artifact.SetPayload(out)
}

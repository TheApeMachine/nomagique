package vector

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

	if err != nil || len(payload) < 8 {
		return 0, false
	}

	return math.Float64frombits(binary.BigEndian.Uint64(payload[:8])), true
}

func putFloat64Payload(artifact **datura.Artifact, name string, value float64) {
	*artifact = datura.Acquire(name, datura.Artifact_Type_json)
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, math.Float64bits(value))
	_ = (*artifact).SetPayload(payload)
}

package learning

import (
	"encoding/binary"
	"errors"
	"math"

	"github.com/theapemachine/datura"
)

var (
	ErrEmptyInputs   = errors.New("learning: empty inputs")
	ErrZeroPredicted = errors.New("learning: zero predicted value")
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

func putFloat64Payload(artifact **datura.Artifact, name string, value float64) {
	*artifact = datura.Acquire(name, datura.Artifact_Type_json)
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, math.Float64bits(value))
	_ = (*artifact).SetPayload(payload)
}

func parsePredictedActual(
	primary float64, extras []float64,
) (float64, float64, error) {
	if len(extras) >= 2 {
		predicted := extras[0]
		actual := extras[1]

		if predicted == 0 {
			return 0, 0, ErrZeroPredicted
		}

		return predicted, actual, nil
	}

	if len(extras) == 0 {
		return 0, 0, ErrEmptyInputs
	}

	predicted := primary
	actual := extras[0]

	if predicted == 0 {
		return 0, 0, ErrZeroPredicted
	}

	return predicted, actual, nil
}

package statistic

import (
	"encoding/binary"
	"math"

	"github.com/theapemachine/datura"
	"gonum.org/v1/gonum/floats"
)

/*
Min returns the smallest value in a batch passed to Read.
*/
type Min struct {
	artifact *datura.Artifact
}

/*
NewMin creates a min stage.
*/
func NewMin() *Min {
	return &Min{
		artifact: datura.Acquire("min", datura.Artifact_Type_json),
	}
}

func (min *Min) Write(p []byte) (int, error) {
	return min.artifact.Write(p)
}

func (min *Min) Read(p []byte) (int, error) {
	payload, err := min.artifact.Payload()

	if err == nil && len(payload) >= 8 && len(payload)%8 == 0 {
		count := len(payload) / 8
		values := make([]float64, count)

		for index := range count {
			offset := index * 8
			values[index] = math.Float64frombits(binary.BigEndian.Uint64(payload[offset : offset+8]))
		}

		putFloat64Payload(&min.artifact, "min", floats.Min(values))
	}

	return min.artifact.Read(p)
}

func (min *Min) Close() error {
	return nil
}

func (min *Min) Reset() error {
	return nil
}

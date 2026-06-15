package statistic

import (
	"encoding/binary"
	"math"

	"github.com/theapemachine/datura"
	"gonum.org/v1/gonum/floats"
)

/*
Max returns the largest value in a batch passed to Read.
*/
type Max struct {
	artifact *datura.Artifact
}

/*
NewMax creates a max stage.
*/
func NewMax() *Max {
	return &Max{
		artifact: datura.Acquire("max", datura.Artifact_Type_json),
	}
}

func (max *Max) Write(p []byte) (int, error) {
	return max.artifact.Write(p)
}

func (max *Max) Read(p []byte) (int, error) {
	payload, err := max.artifact.Payload()

	if err == nil && len(payload) >= 8 && len(payload)%8 == 0 {
		count := len(payload) / 8
		values := make([]float64, count)

		for index := range count {
			offset := index * 8
			values[index] = math.Float64frombits(binary.BigEndian.Uint64(payload[offset : offset+8]))
		}

		putFloat64Payload(&max.artifact, "max", floats.Max(values))
	}

	return max.artifact.Read(p)
}

func (max *Max) Close() error {
	return nil
}

func (max *Max) Reset() error {
	return nil
}

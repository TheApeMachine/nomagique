package statistic

import (
	"encoding/binary"
	"math"

	"github.com/theapemachine/datura"
	"gonum.org/v1/gonum/floats"
)

/*
Sum adds every sample in one Read call.
*/
type Sum struct {
	artifact *datura.Artifact
}

/*
NewSum creates a sum stage.
*/
func NewSum() *Sum {
	return &Sum{
		artifact: datura.Acquire("sum", datura.Artifact_Type_json),
	}
}

func (sum *Sum) Write(p []byte) (int, error) {
	return sum.artifact.Write(p)
}

func (sum *Sum) Read(p []byte) (int, error) {
	payload, err := sum.artifact.Payload()

	if err == nil && len(payload) >= 8 && len(payload)%8 == 0 {
		count := len(payload) / 8
		values := make([]float64, count)

		for index := range count {
			offset := index * 8
			values[index] = math.Float64frombits(binary.BigEndian.Uint64(payload[offset : offset+8]))
		}

		putFloat64Payload(&sum.artifact, "sum", floats.Sum(values))
	}

	return sum.artifact.Read(p)
}

func (sum *Sum) Close() error {
	return nil
}

func (sum *Sum) Reset() error {
	return nil
}

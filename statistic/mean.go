package statistic

import (
	"encoding/binary"
	"math"

	"github.com/theapemachine/datura"
	"gonum.org/v1/gonum/stat"
)

/*
Mean computes the arithmetic average of every sample in one Read call.
*/
type Mean struct {
	artifact *datura.Artifact
	weights  []float64
}

/*
NewMean creates a mean stage.
*/
func NewMean(weights []float64) *Mean {
	return &Mean{
		artifact: datura.Acquire("mean", datura.Artifact_Type_json),
		weights:  weights,
	}
}

func (mean *Mean) Write(p []byte) (int, error) {
	return mean.artifact.Write(p)
}

func (mean *Mean) Read(p []byte) (int, error) {
	payload, err := mean.artifact.Payload()

	if err == nil && len(payload) >= 8 && len(payload)%8 == 0 {
		count := len(payload) / 8
		values := make([]float64, count)

		for index := range count {
			offset := index * 8
			values[index] = math.Float64frombits(binary.BigEndian.Uint64(payload[offset : offset+8]))
		}

		weights := mean.weights

		if len(weights) == 0 {
			weights = nil
		}

		putFloat64Payload(&mean.artifact, "mean", stat.Mean(values, weights))
	}

	return mean.artifact.Read(p)
}

func (mean *Mean) Close() error {
	return nil
}

func (mean *Mean) Reset() error {
	mean.weights = nil

	return nil
}

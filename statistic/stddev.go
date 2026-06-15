package statistic

import (
	"encoding/binary"
	"math"

	"github.com/theapemachine/datura"
	"gonum.org/v1/gonum/stat"
)

/*
StdDev computes the sample standard deviation of a stream of numbers.
*/
type StdDev struct {
	artifact *datura.Artifact
	weights  []float64
}

/*
NewStdDev creates a standard-deviation stage.
*/
func NewStdDev(weights []float64) *StdDev {
	return &StdDev{
		artifact: datura.Acquire("stddev", datura.Artifact_Type_json),
		weights:  weights,
	}
}

func (stdDev *StdDev) Write(p []byte) (int, error) {
	return stdDev.artifact.Write(p)
}

func (stdDev *StdDev) Read(p []byte) (int, error) {
	payload, err := stdDev.artifact.Payload()

	if err == nil && len(payload) >= 8 && len(payload)%8 == 0 {
		count := len(payload) / 8
		values := make([]float64, count)

		for index := range count {
			offset := index * 8
			values[index] = math.Float64frombits(binary.BigEndian.Uint64(payload[offset : offset+8]))
		}

		if count < 2 {
			putFloat64Payload(&stdDev.artifact, "stddev", 0)

			return stdDev.artifact.Read(p)
		}

		weights := stdDev.weights

		if len(weights) == 0 {
			weights = nil
		}

		putFloat64Payload(&stdDev.artifact, "stddev", stat.StdDev(values, weights))
	}

	return stdDev.artifact.Read(p)
}

func (stdDev *StdDev) Close() error {
	return nil
}

func (stdDev *StdDev) Reset() error {
	stdDev.weights = nil

	return nil
}

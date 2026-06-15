package statistic

import (
	"encoding/binary"
	"math"

	"github.com/theapemachine/datura"
)

/*
MedianAbsolute measures typical magnitude while ignoring sign.
*/
type MedianAbsolute struct {
	artifact *datura.Artifact
	weights  []float64
}

/*
NewMedianAbsolute creates a median-absolute stage.
*/
func NewMedianAbsolute(weights []float64) *MedianAbsolute {
	return &MedianAbsolute{
		artifact: datura.Acquire("median_absolute", datura.Artifact_Type_json),
		weights:  weights,
	}
}

func (medianAbsolute *MedianAbsolute) Write(p []byte) (int, error) {
	return medianAbsolute.artifact.Write(p)
}

func (medianAbsolute *MedianAbsolute) Read(p []byte) (int, error) {
	payload, err := medianAbsolute.artifact.Payload()

	if err != nil || len(payload) < 8 || len(payload)%8 != 0 {
		return medianAbsolute.artifact.Read(p)
	}

	count := len(payload) / 8
	values := make([]float64, count)

	for index := range count {
		offset := index * 8
		values[index] = math.Float64frombits(binary.BigEndian.Uint64(payload[offset : offset+8]))
	}

	if len(values) == 0 {
		return medianAbsolute.artifact.Read(p)
	}

	absoluteValues := make([]float64, len(values))

	for index, value := range values {
		absoluteValues[index] = math.Abs(value)
	}

	putFloat64Payload(&medianAbsolute.artifact, "median_absolute", MedianOf(absoluteValues))

	return medianAbsolute.artifact.Read(p)
}

func (medianAbsolute *MedianAbsolute) Close() error {
	return nil
}

func (medianAbsolute *MedianAbsolute) Reset() error {
	medianAbsolute.weights = nil

	return nil
}

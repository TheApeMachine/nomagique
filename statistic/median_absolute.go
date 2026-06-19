package statistic

import (
	"math"
	"sort"

	"github.com/theapemachine/datura"
)

/*
MedianAbsolute measures typical magnitude while ignoring sign.
*/
type MedianAbsolute struct {
	artifact *datura.Artifact
}

/*
NewMedianAbsolute creates a median-absolute stage.
*/
func NewMedianAbsolute() *MedianAbsolute {
	return &MedianAbsolute{
		artifact: datura.Acquire("median_absolute", datura.APPJSON),
	}
}

func (medianAbsolute *MedianAbsolute) Write(p []byte) (int, error) {
	return medianAbsolute.artifact.Write(p)
}

func (medianAbsolute *MedianAbsolute) Read(p []byte) (int, error) {
	sample := datura.Peek[float64](medianAbsolute.artifact, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return medianAbsolute.artifact.Read(p)
	}

	history := datura.Peek[[]float64](medianAbsolute.artifact, "history")
	history = append(history, sample)
	medianAbsolute.artifact.Poke(history, "history")

	value := MedianAbsoluteOf(history)
	medianAbsolute.artifact.Poke(datura.Map[float64]{"value": value}, "output")

	return medianAbsolute.artifact.Read(p)
}

func (medianAbsolute *MedianAbsolute) Close() error {
	return nil
}

/*
MedianAbsoluteOf returns the median of absolute values.
*/
func MedianAbsoluteOf(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	absoluteValues := make([]float64, len(values))

	for index, value := range values {
		absoluteValues[index] = math.Abs(value)
	}

	return MedianOf(absoluteValues)
}

/*
SpanOf returns the range between smallest and largest sample values.
*/
func SpanOf(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	return sorted[len(sorted)-1] - sorted[0]
}

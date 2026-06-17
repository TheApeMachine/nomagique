package statistic

import (
	"encoding/binary"
	"math"
	"sort"

	"github.com/theapemachine/datura"
	"gonum.org/v1/gonum/stat"
)

func putFloat64Payload(artifact **datura.Artifact, name string, value float64) {
	*artifact = datura.Acquire(name, datura.Artifact_Type_json)
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, math.Float64bits(value))
	_ = (*artifact).SetPayload(payload)
}

/*
QuantileOf returns the sample quantile with linear interpolation.
*/
func QuantileOf(percentile float64, values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	return stat.Quantile(percentile, stat.LinInterp, sorted, nil)
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

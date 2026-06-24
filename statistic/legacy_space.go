package statistic

import (
	"math"
	"sort"

	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
QuantileOf returns the sample quantile with linear interpolation.
ponytail: legacy scalar helper; upgrade path is NewQuantile Write/Read at call sites.
*/
func QuantileOf(percentile float64, values []float64) (float64, error) {
	if len(values) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"quantile: values required",
			nil,
		))
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	for _, value := range sorted {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"quantile: sample is non-finite",
				nil,
			))
		}
	}

	return stat.Quantile(percentile, stat.LinInterp, sorted, nil), nil
}

/*
LinSpace returns count evenly spaced values from start through end inclusive.
ponytail: legacy grid helper; upgrade path is a dedicated Linspace stage or inline at caller.
*/
func LinSpace(start, end float64, count int) ([]float64, error) {
	if count <= 0 {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"linspace: count must be positive",
			nil,
		))
	}

	if count == 1 {
		return []float64{start}, nil
	}

	values := make([]float64, count)
	step := (end - start) / float64(count-1)

	for index := range values {
		values[index] = start + step*float64(index)
	}

	return values, nil
}

/*
LogSpace returns count values logarithmically spaced between start and end.
ponytail: legacy grid helper; upgrade path is a dedicated Logspace stage or inline at caller.
*/
func LogSpace(start, end float64, count int) ([]float64, error) {
	if count <= 0 || start <= 0 || end <= 0 {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"logspace: count and endpoints must be positive",
			nil,
		))
	}

	if count == 1 {
		return []float64{start}, nil
	}

	logStart := math.Log(start)
	logEnd := math.Log(end)
	step := (logEnd - logStart) / float64(count-1)
	values := make([]float64, count)

	for index := range values {
		values[index] = math.Exp(logStart + step*float64(index))
	}

	return values, nil
}

/*
Quartiles returns the lower and upper quartiles of values.
ponytail: legacy scalar helper; upgrade path is NewQuantile Write/Read at call sites.
*/
func Quartiles(values []float64) (lower float64, upper float64, err error) {
	if len(values) == 0 {
		return 0, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"quartiles: values required",
			nil,
		))
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	for _, value := range sorted {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"quartiles: sample is non-finite",
				nil,
			))
		}
	}

	lower = stat.Quantile(0.25, stat.LinInterp, sorted, nil)
	upper = stat.Quantile(0.75, stat.LinInterp, sorted, nil)

	return lower, upper, nil
}

/*
SpanOf returns the count of distinct sample levels in values.
ponytail: legacy scalar helper; upgrade path is inline distinct-count at caller.
*/
func SpanOf(values []float64) (float64, error) {
	if len(values) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"span: values required",
			nil,
		))
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	distinct := 1

	for index := 1; index < len(sorted); index++ {
		if sorted[index] == sorted[index-1] {
			continue
		}

		distinct++
	}

	if distinct <= 1 {
		return 0, nil
	}

	return float64(distinct), nil
}

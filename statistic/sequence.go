package statistic

import (
	"math"
	"sort"

	"gonum.org/v1/gonum/stat"
)

/*
LinSpace returns count evenly spaced values from start through end inclusive.
*/
func LinSpace(start, end float64, count int) []float64 {
	if count <= 0 {
		return nil
	}

	if count == 1 {
		return []float64{start}
	}

	values := make([]float64, count)
	step := (end - start) / float64(count-1)

	for index := range values {
		values[index] = start + step*float64(index)
	}

	return values
}

/*
LogSpace returns count values logarithmically spaced between start and end.
*/
func LogSpace(start, end float64, count int) []float64 {
	if count <= 0 || start <= 0 || end <= 0 {
		return nil
	}

	if count == 1 {
		return []float64{start}
	}

	logStart := math.Log(start)
	logEnd := math.Log(end)
	step := (logEnd - logStart) / float64(count-1)
	values := make([]float64, count)

	for index := range values {
		values[index] = math.Exp(logStart + step*float64(index))
	}

	return values
}

/*
Quartiles returns the lower and upper quartiles of values.
*/
func Quartiles(values []float64) (lower float64, upper float64) {
	if len(values) == 0 {
		return 0, 0
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	lower = stat.Quantile(0.25, stat.LinInterp, sorted, nil)
	upper = stat.Quantile(0.75, stat.LinInterp, sorted, nil)

	return lower, upper
}

package statistic

import (
	"math"
	"sort"
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

	lower = quantileSorted(sorted, 0.25)
	upper = quantileSorted(sorted, 0.75)

	return lower, upper
}

func quantileSorted(sorted []float64, percentile float64) float64 {
	if len(sorted) == 0 {
		return 0
	}

	if percentile <= 0 {
		return sorted[0]
	}

	if percentile >= 1 {
		return sorted[len(sorted)-1]
	}

	position := percentile * float64(len(sorted)-1)
	lowerIndex := int(math.Floor(position))
	upperIndex := int(math.Ceil(position))

	if lowerIndex == upperIndex {
		return sorted[lowerIndex]
	}

	weight := position - float64(lowerIndex)

	return sorted[lowerIndex]*(1-weight) + sorted[upperIndex]*weight
}

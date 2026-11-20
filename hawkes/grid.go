package hawkes

import (
	"math"

	"github.com/theapemachine/errnie"
)

/*
Feeds algorithm/excitation and equation Hawkes stages — linspace turns marked trade arrivals into fitted intensities and categories.
*/
func linspace(start, end float64, count int) ([]float64, error) {
	if count <= 0 {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"hawkes grid: linspace count must be positive",
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
Feeds algorithm/excitation and equation Hawkes stages — logspace fits marked trade arrivals into intensities and fit categories.
*/

func logspace(start, end float64, count int) ([]float64, error) {
	if count <= 0 || start <= 0 || end <= 0 {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"hawkes grid: logspace count and endpoints must be positive",
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

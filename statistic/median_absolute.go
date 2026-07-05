package statistic

import (
	"math"

	"github.com/theapemachine/errnie"
)

/*
MedianAbsolute computes the median of absolute values over retained history.
*/
type MedianAbsolute struct {
	history []float64
}

/*
NewMedianAbsolute returns a typed median-absolute accumulator.
*/
func NewMedianAbsolute() *MedianAbsolute {
	return &MedianAbsolute{}
}

/*
Measure adds one sample and returns the median of retained absolute values.
*/
func (medianAbsolute *MedianAbsolute) Measure(sample float64) (ScalarOutput, error) {
	if err := finiteStatistic("median-absolute", sample); err != nil {
		return ScalarOutput{}, err
	}

	medianAbsolute.history = append(medianAbsolute.history, math.Abs(sample))
	value, ok := MedianOf(medianAbsolute.history)

	if !ok {
		return ScalarOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"median-absolute: unable to compute median",
			nil,
		))
	}

	return ScalarOutput{
		Value: value,
		Ready: true,
		Count: len(medianAbsolute.history),
	}, nil
}

/*
MedianAbsoluteOf returns the median of absolute values.
*/
func MedianAbsoluteOf(values []float64) (float64, bool) {
	if len(values) == 0 {
		return 0, false
	}

	absolute := make([]float64, len(values))

	for index, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, false
		}

		absolute[index] = math.Abs(value)
	}

	return MedianOf(absolute)
}

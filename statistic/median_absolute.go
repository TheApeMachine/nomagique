package statistic

import (
	"math"

	"github.com/theapemachine/nomagique/core"
)

/*
MedianAbsolute measures typical magnitude while ignoring sign.

Each input is converted to its absolute value, then Median runs on that batch.
*/
type MedianAbsolute[T ~float64] struct {
	weights []float64
	output  core.Scalar[T]
}

/*
NewMedianAbsolute creates a median-absolute stage.
*/
func NewMedianAbsolute[T ~float64](weights []float64) *MedianAbsolute[T] {
	return &MedianAbsolute[T]{
		weights: weights,
	}
}

/*
Observe returns the median absolute value of the input stream.
*/
func (medianAbsolute *MedianAbsolute[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	values := sampleBatch[T](inputs...)

	if len(values) == 0 {
		return medianAbsolute.output
	}

	absolute := make([]core.Number[T], len(values))

	for index, value := range values {
		absolute[index] = core.Scalar[T](T(math.Abs(value)))
	}

	medianAbsolute.output = NewMedian[T](medianAbsolute.weights).Observe(absolute...)

	return medianAbsolute.output
}

func (medianAbsolute *MedianAbsolute[T]) Reset() error {
	medianAbsolute.weights = nil
	medianAbsolute.output = core.Scalar[T](0)

	return nil
}

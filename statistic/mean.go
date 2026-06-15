package statistic

import (
	"github.com/theapemachine/nomagique/core"
	"gonum.org/v1/gonum/stat"
)

/*
Mean computes the arithmetic average of every sample in one Observe call.

Optional weights emphasize selected samples. Compose inside nomagique.Number(...)
or call Observe directly with boundary scalars.
*/
type Mean[T ~float64] struct {
	weights []float64
	output  core.Scalar[T]
}

/*
NewMean creates a mean stage.
*/
func NewMean[T ~float64](weights []float64) *Mean[T] {
	return &Mean[T]{
		weights: weights,
	}
}

/*
Observe computes the mean of a stream of numbers.
*/
func (mean *Mean[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	values := sampleBatch[T](inputs...)

	if len(values) == 0 {
		return mean.output
	}

	weights := mean.weights

	if len(weights) == 0 {
		weights = nil
	}

	mean.output = core.Scalar[T](T(stat.Mean(values, weights)))

	return mean.output
}

func (mean *Mean[T]) Reset() error {
	mean.weights = nil
	mean.output = core.Scalar[T](0)

	return nil
}

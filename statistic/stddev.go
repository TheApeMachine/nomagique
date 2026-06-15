package statistic

import (
	"github.com/theapemachine/nomagique/core"
	"gonum.org/v1/gonum/stat"
)

/*
StdDev computes the sample standard deviation of a stream of numbers.

Optional weights turn the stream into a weighted dispersion when some
observations carry more evidence than others.
*/
type StdDev[T ~float64] struct {
	weights []float64
	output  core.Scalar[T]
}

/*
NewStdDev creates a standard-deviation stage.
*/
func NewStdDev[T ~float64](weights []float64) *StdDev[T] {
	return &StdDev[T]{
		weights: weights,
	}
}

/*
Observe returns the sample standard deviation of the input stream.
*/
func (stdDev *StdDev[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	values := sampleBatch[T](inputs...)

	if len(values) == 0 {
		return stdDev.output
	}

	weights := stdDev.weights

	if len(weights) == 0 {
		weights = nil
	}

	stdDev.output = core.Scalar[T](T(stat.StdDev(values, weights)))

	return stdDev.output
}

func (stdDev *StdDev[T]) Reset() error {
	stdDev.weights = nil
	stdDev.output = core.Scalar[T](0)

	return nil
}

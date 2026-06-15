package statistic

import (
	"github.com/theapemachine/nomagique/core"
	"gonum.org/v1/gonum/floats"
)

/*
Sum adds every sample in one Observe call.

Example: three samples (1.2, 0.8, 3.0) sum to 5.0. There is no memory between
calls — each Observe is a fresh total over its inputs.
*/
type Sum[T ~float64] struct {
	output core.Scalar[T]
}

/*
NewSum creates a sum stage.
*/
func NewSum[T ~float64]() *Sum[T] {
	return &Sum[T]{}
}

/*
Observe returns the sum of the input stream.
*/
func (sum *Sum[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	values := sampleBatch[T](inputs...)

	if len(values) == 0 {
		return sum.output
	}

	sum.output = core.Scalar[T](T(floats.Sum(values)))

	return sum.output
}

func (sum *Sum[T]) Reset() error {
	sum.output = core.Scalar[T](0)

	return nil
}

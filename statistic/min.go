package statistic

import (
	"github.com/theapemachine/nomagique/core"
	"gonum.org/v1/gonum/floats"
)

/*
Min returns the smallest value in a batch passed to Observe.

Stateless snapshot reducer — useful for floor liquidity, minimum spread, or any
best-case-in-set step. Min implements core.Number. Empty input returns zero.
*/
type Min[T ~float64] struct {
	output core.Scalar[T]
}

/*
NewMin creates a min stage.
*/
func NewMin[T ~float64]() *Min[T] {
	return &Min[T]{}
}

/*
Observe returns the minimum of the input stream.
*/
func (min *Min[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	values := sampleBatch[T](inputs...)

	if len(values) == 0 {
		return min.output
	}

	min.output = core.Scalar[T](T(floats.Min(values)))

	return min.output
}

func (min *Min[T]) Reset() error {
	min.output = core.Scalar[T](0)

	return nil
}

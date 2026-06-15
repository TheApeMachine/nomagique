package statistic

import (
	"github.com/theapemachine/nomagique/core"
	"gonum.org/v1/gonum/floats"
)

/*
Max returns the largest value in a batch passed to Observe.

Like Min, it is a stateless snapshot reducer over whatever scalars you feed it in
one call — useful for peak energy, best bid depth, or any worst case in this set.
Max implements core.Number. Empty input returns zero.
*/
type Max[T ~float64] struct {
	output core.Scalar[T]
}

/*
NewMax creates a max stage.
*/
func NewMax[T ~float64]() *Max[T] {
	return &Max[T]{}
}

/*
Observe returns the maximum of the input stream.
*/
func (max *Max[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	values := sampleBatch[T](inputs...)

	if len(values) == 0 {
		return max.output
	}

	max.output = core.Scalar[T](T(floats.Max(values)))

	return max.output
}

func (max *Max[T]) Reset() error {
	max.output = core.Scalar[T](0)

	return nil
}

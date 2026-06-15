package logic

import (
	"math"

	"github.com/theapemachine/nomagique/core"
)

/*
Compare emits the signed difference between two scalar operands.
*/
type Compare[T ~float64] struct {
	output core.Scalar[T]
}

/*
NewCompare creates a binary comparator stage.
*/
func NewCompare[T ~float64]() *Compare[T] {
	return &Compare[T]{}
}

/*
Observe ingests left and right scalars and returns sign(left-right).
*/
func (compare *Compare[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if len(inputs) == 0 {
		return compare.output
	}

	scalars, ok := collectScalars(inputs...)

	if !ok || len(scalars) < 2 {
		return compare.output
	}

	left := scalars[0]
	right := scalars[1]
	delta := left - right

	if delta == 0 {
		compare.output = core.Scalar[T](0)

		return compare.output
	}

	compare.output = core.Scalar[T](T(math.Copysign(1, delta)))

	return compare.output
}

/*
Reset clears the last emitted value.
*/
func (compare *Compare[T]) Reset() error {
	compare.output = core.Scalar[T](0)

	return nil
}

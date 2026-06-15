package logic

import (
	"slices"

	"github.com/theapemachine/nomagique/core"
)

/*
And emits truth when every observed scalar is truthy.
*/
type And[T ~float64] struct {
	output core.Scalar[T]
}

/*
NewAnd creates an n-input conjunction gate.
*/
func NewAnd[T ~float64]() *And[T] {
	return &And[T]{}
}

/*
Observe ingests one or more scalars and returns 1 when all are truthy.
*/
func (andGate *And[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if len(inputs) == 0 {
		return andGate.output
	}

	scalars, ok := collectScalars(inputs...)

	if !ok || len(scalars) == 0 {
		return andGate.output
	}

	for _, sample := range scalars {
		if !truthy(sample) {
			andGate.output = core.Scalar[T](0)

			return andGate.output
		}
	}

	andGate.output = core.Scalar[T](1)

	return andGate.output
}

/*
Reset clears the last emitted value.
*/
func (andGate *And[T]) Reset() error {
	andGate.output = core.Scalar[T](0)

	return nil
}

/*
Or emits truth when any observed scalar is truthy.
*/
type Or[T ~float64] struct {
	output core.Scalar[T]
}

/*
NewOr creates an n-input disjunction gate.
*/
func NewOr[T ~float64]() *Or[T] {
	return &Or[T]{}
}

/*
Observe ingests one or more scalars and returns 1 when any is truthy.
*/
func (orGate *Or[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if len(inputs) == 0 {
		return orGate.output
	}

	scalars, ok := collectScalars[T](inputs...)

	if !ok || len(scalars) == 0 {
		return orGate.output
	}

	if slices.ContainsFunc(scalars, truthy) {
			orGate.output = core.Scalar[T](1)

			return orGate.output
		}

	orGate.output = core.Scalar[T](0)

	return orGate.output
}

/*
Reset clears the last emitted value.
*/
func (orGate *Or[T]) Reset() error {
	orGate.output = core.Scalar[T](0)

	return nil
}

/*
Not inverts the truthiness of a single scalar input.
*/
type Not[T ~float64] struct {
	output core.Scalar[T]
}

/*
NewNot creates a unary inversion gate.
*/
func NewNot[T ~float64]() *Not[T] {
	return &Not[T]{}
}

/*
Observe ingests one scalar and returns 1 when it is not truthy.
*/
func (notGate *Not[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if len(inputs) == 0 {
		return notGate.output
	}

	scalars, ok := collectScalars[T](inputs...)

	if !ok || len(scalars) == 0 {
		return notGate.output
	}

	if truthy(scalars[0]) {
		notGate.output = core.Scalar[T](0)

		return notGate.output
	}

	notGate.output = core.Scalar[T](1)

	return notGate.output
}

/*
Reset clears the last emitted value.
*/
func (notGate *Not[T]) Reset() error {
	notGate.output = core.Scalar[T](0)

	return nil
}

/*
Xor emits truth when an odd number of inputs are truthy.
*/
type Xor[T ~float64] struct {
	output core.Scalar[T]
}

/*
NewXor creates an n-input exclusive-or gate.
*/
func NewXor[T ~float64]() *Xor[T] {
	return &Xor[T]{}
}

/*
Observe ingests one or more scalars and returns 1 when an odd count is truthy.
*/
func (xorGate *Xor[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if len(inputs) == 0 {
		return xorGate.output
	}

	scalars, ok := collectScalars(inputs...)

	if !ok || len(scalars) == 0 {
		return xorGate.output
	}

	truthCount := 0

	for _, sample := range scalars {
		if truthy(sample) {
			truthCount++
		}
	}

	xorGate.output = core.Scalar[T](T(float64(truthCount % 2)))

	return xorGate.output
}

/*
Reset clears the last emitted value.
*/
func (xorGate *Xor[T]) Reset() error {
	xorGate.output = core.Scalar[T](0)

	return nil
}

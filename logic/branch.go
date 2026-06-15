package logic

import (
	"math"

	"github.com/theapemachine/nomagique/core"
)

/*
Select passes the consequent scalar when the condition is truthy, otherwise the alternative.
*/
type Select[T ~float64] struct {
	output core.Scalar[T]
}

/*
NewSelect creates a binary branch stage.
*/
func NewSelect[T ~float64]() *Select[T] {
	return &Select[T]{}
}

/*
Observe ingests condition, consequent, and alternative scalars in that order.
*/
func (selectStage *Select[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if len(inputs) == 0 {
		return selectStage.output
	}

	scalars, ok := collectScalars(inputs...)

	if !ok || len(scalars) < 3 {
		return selectStage.output
	}

	condition := scalars[0]
	consequent := scalars[1]
	alternative := scalars[2]

	if truthy(condition) {
		selectStage.output = core.Scalar[T](T(consequent))

		return selectStage.output
	}

	selectStage.output = core.Scalar[T](T(alternative))

	return selectStage.output
}

/*
Reset clears the last emitted value.
*/
func (selectStage *Select[T]) Reset() error {
	selectStage.output = core.Scalar[T](0)

	return nil
}

/*
Gate passes the signal scalar when the enable input is truthy.
*/
type Gate[T ~float64] struct {
	output core.Scalar[T]
}

/*
NewGate creates an enable-gated pass-through stage.
*/
func NewGate[T ~float64]() *Gate[T] {
	return &Gate[T]{}
}

/*
Observe ingests enable and signal scalars in that order.
*/
func (gate *Gate[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if len(inputs) == 0 {
		return gate.output
	}

	scalars, ok := collectScalars[T](inputs...)

	if !ok || len(scalars) < 2 {
		return gate.output
	}

	enable := scalars[0]
	signal := scalars[1]

	if !truthy(enable) {
		gate.output = core.Scalar[T](0)

		return gate.output
	}

	gate.output = core.Scalar[T](T(signal))

	return gate.output
}

/*
Reset clears the last emitted value.
*/
func (gate *Gate[T]) Reset() error {
	gate.output = core.Scalar[T](0)

	return nil
}

/*
Mux selects one of several value scalars using a selector index.
*/
type Mux[T ~float64] struct {
	routeCount int
	output     core.Scalar[T]
}

/*
NewMux creates an n-way multiplexer expecting selector plus routeCount values.
*/
func NewMux[T ~float64](routeCount int) *Mux[T] {
	return &Mux[T]{
		routeCount: routeCount,
	}
}

/*
Observe ingests selector followed by routeCount value scalars.
*/
func (mux *Mux[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if len(inputs) == 0 {
		return mux.output
	}

	scalars, ok := collectScalars(inputs...)

	if !ok || len(scalars) < mux.routeCount+1 {
		return mux.output
	}

	selector := int(math.Round(scalars[0]))

	if selector < 0 || selector >= mux.routeCount {
		return mux.output
	}

	mux.output = core.Scalar[T](T(scalars[1+selector]))

	return mux.output
}

/*
Reset clears the last emitted value.
*/
func (mux *Mux[T]) Reset() error {
	mux.output = core.Scalar[T](0)

	return nil
}

/*
FirstMatch walks when/then pairs and returns the first consequent whose condition is truthy.
*/
type FirstMatch[T ~float64] struct {
	output core.Scalar[T]
}

/*
NewFirstMatch creates a multi-branch selector over paired conditions and values.
*/
func NewFirstMatch[T ~float64]() *FirstMatch[T] {
	return &FirstMatch[T]{}
}

/*
Observe ingests when/then pairs followed by a default scalar.
*/
func (firstMatch *FirstMatch[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if len(inputs) == 0 {
		return firstMatch.output
	}

	scalars, ok := collectScalars[T](inputs...)

	if !ok || len(scalars) < 3 {
		return firstMatch.output
	}

	defaultValue := scalars[len(scalars)-1]
	pairCount := (len(scalars) - 1) / 2

	for pairIndex := range pairCount {
		when := scalars[2*pairIndex]
		then := scalars[2*pairIndex+1]

		if truthy(when) {
			firstMatch.output = core.Scalar[T](T(then))

			return firstMatch.output
		}
	}

	firstMatch.output = core.Scalar[T](T(defaultValue))

	return firstMatch.output
}

/*
Reset clears the last emitted value.
*/
func (firstMatch *FirstMatch[T]) Reset() error {
	firstMatch.output = core.Scalar[T](0)

	return nil
}

/*
Latch holds the last captured signal while the enable input is truthy.
*/
type Latch[T ~float64] struct {
	held   float64
	output core.Scalar[T]
}

/*
NewLatch creates a hold stage that captures on enable.
*/
func NewLatch[T ~float64]() *Latch[T] {
	return &Latch[T]{}
}

/*
Observe ingests enable and signal scalars in that order.
*/
func (latch *Latch[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if len(inputs) == 0 {
		return latch.output
	}

	scalars, ok := collectScalars[T](inputs...)

	if !ok || len(scalars) < 2 {
		return latch.output
	}

	enable := scalars[0]
	signal := scalars[1]

	if truthy(enable) {
		latch.held = signal
	}

	latch.output = core.Scalar[T](T(latch.held))

	return latch.output
}

/*
Reset clears held and emitted values.
*/
func (latch *Latch[T]) Reset() error {
	latch.held = 0
	latch.output = core.Scalar[T](0)

	return nil
}

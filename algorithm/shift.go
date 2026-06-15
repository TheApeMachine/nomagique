package algorithm

import (
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/statistic"
)

/*
Shift measures distribution drift between a reference stream and a live stream via KL divergence.
*/
type Shift[T ~float64] struct {
	reference  []float64
	live       []float64
	weights    []float64
	divergence *statistic.KLDivergence[T]
	output     core.Scalar[T]
}

/*
NewShift creates a distribution-shift dynamic over reference and live streams.
expectedSum and floor are forwarded to statistic.KLDivergence; zero values are derived per call.
*/
func NewShift[T ~float64](
	reference, live, weights []float64,
	expectedSum, floor float64,
) *Shift[T] {
	return &Shift[T]{
		reference:  reference,
		live:       live,
		weights:    weights,
		divergence: statistic.NewKLDivergence[T](weights, expectedSum, floor),
	}
}

/*
Observe returns KL divergence from reference (expected) to live (observed).
*/
func (shift *Shift[T]) Observe(_ ...core.Number[T]) core.Scalar[T] {
	reference := shift.reference
	live := shift.live

	if len(reference) == 0 || len(live) == 0 {
		return shift.output
	}

	inputs := append(
		samplesToInputs[T](live),
		samplesToInputs[T](reference)...,
	)

	shift.output = shift.divergence.Observe(inputs...)

	return shift.output
}

/*
Reset clears derived state.
*/
func (shift *Shift[T]) Reset() error {
	shift.weights = nil
	shift.output = core.Scalar[T](0)

	return shift.divergence.Reset()
}

package algorithm

import (
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/statistic"
)

/*
Shift measures distribution drift between a reference stream and a live stream via KL divergence.
*/
type Shift struct {
	reference  core.Numbers
	live       core.Numbers
	weights    core.Numbers
	divergence *statistic.KLDivergence
}

/*
NewShift creates a distribution-shift dynamic over reference and live streams.
expectedSum and floor are forwarded to statistic.KLDivergence; zero values are derived per call.
*/
func NewShift(
	reference, live, weights core.Numbers,
	expectedSum, floor float64,
) *Shift {
	return &Shift{
		reference:  reference,
		live:       live,
		weights:    weights,
		divergence: statistic.NewKLDivergence(weights, expectedSum, floor),
	}
}

/*
Observe returns KL divergence from reference (expected) to live (observed).
*/
func (shift *Shift) Observe(_ ...core.Number) core.Float64 {
	reference := nomagique.Samples(shift.reference)
	live := nomagique.Samples(shift.live)

	if len(reference) == 0 || len(live) == 0 {
		return 0
	}

	inputs := append(
		nomagique.Numbers(live...),
		nomagique.Numbers(reference...)...,
	)

	return shift.divergence.Observe(inputs...)
}

/*
Reset clears derived state.
*/
func (shift *Shift) Reset() error {
	shift.weights = nil

	return shift.divergence.Reset()
}

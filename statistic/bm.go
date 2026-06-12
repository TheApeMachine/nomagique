package statistic

import (
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/core"
	"gonum.org/v1/gonum/stat"
)

/*
BivariateMoment computes E[(x - mu_x)^r (y - mu_y)^s] for two configured streams.
*/
type BivariateMoment struct {
	r       float64
	s       float64
	x       core.Numbers
	y       core.Numbers
	weights core.Numbers
}

/*
NewBivariateMoment creates a bivariate moment dynamic at exponents r and s.
*/
func NewBivariateMoment(
	r, s float64, x, y, weights core.Numbers,
) *BivariateMoment {
	return &BivariateMoment{
		r:       r,
		s:       s,
		x:       x,
		y:       y,
		weights: weights,
	}
}

/*
Powers returns the mixed-moment exponents r and s.
*/
func (bivariateMoment *BivariateMoment) Powers() (r float64, s float64) {
	return bivariateMoment.r, bivariateMoment.s
}

/*
Observe computes the mixed moment for the configured streams.
*/
func (bivariateMoment *BivariateMoment) Observe(inputs ...core.Number) core.Float64 {
	_ = inputs

	xValues := nomagique.Samples(bivariateMoment.x)
	yValues := nomagique.Samples(bivariateMoment.y)
	weights := nomagique.Samples(bivariateMoment.weights)

	if len(weights) == 0 {
		weights = nil
	}

	if len(xValues) != len(yValues) || len(xValues) < 2 {
		errnie.Err(
			errnie.Validation, "unable to compute bivariate moment",
			BivariateMomentError(BivariateMomentErrorInvalidStreams),
		)

		return 0
	}

	if len(weights) != 0 && len(weights) != len(xValues) {
		errnie.Err(
			errnie.Validation, "unable to compute bivariate moment",
			BivariateMomentError(BivariateMomentErrorWeightLengthMismatch),
		)

		return 0
	}

	return core.Float64(
		stat.BivariateMoment(
			bivariateMoment.r, bivariateMoment.s, xValues, yValues, weights,
		),
	)
}

/*
Reset clears derived state.
*/
func (bivariateMoment *BivariateMoment) Reset() error {
	bivariateMoment.weights = nil
	return nil
}

type BivariateMomentErrorType string

const (
	BivariateMomentErrorInvalidStreams       BivariateMomentErrorType = "require aligned streams of at least two samples"
	BivariateMomentErrorWeightLengthMismatch BivariateMomentErrorType = "require equal weight length"
)

type BivariateMomentError string

func (bivariateMomentError BivariateMomentError) Error() string {
	return string(bivariateMomentError)
}

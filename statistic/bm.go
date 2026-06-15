package statistic

import (
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/core"
	"gonum.org/v1/gonum/stat"
)

/*
BivariateMoment computes E[(x - mu_x)^r (y - mu_y)^s] for two configured streams.
*/
type BivariateMoment[T ~float64] struct {
	r       T
	s       T
	x       []float64
	y       []float64
	weights []float64
	output  core.Scalar[T]
}

/*
NewBivariateMoment creates a bivariate moment stage at exponents r and s.
*/
func NewBivariateMoment[T ~float64](
	r, s T, x, y, weights []float64,
) *BivariateMoment[T] {
	return &BivariateMoment[T]{
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
func (bivariateMoment *BivariateMoment[T]) Powers() (r T, s T) {
	return bivariateMoment.r, bivariateMoment.s
}

/*
Observe computes the mixed moment for the configured streams.
*/
func (bivariateMoment *BivariateMoment[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	_ = inputs

	xValues := bivariateMoment.x
	yValues := bivariateMoment.y
	weights := bivariateMoment.weights

	if len(weights) == 0 {
		weights = nil
	}

	if len(xValues) != len(yValues) || len(xValues) < 2 {
		errnie.Err(
			errnie.Validation, "unable to compute bivariate moment",
			BivariateMomentError(BivariateMomentErrorInvalidStreams),
		)

		return bivariateMoment.output
	}

	if len(weights) != 0 && len(weights) != len(xValues) {
		errnie.Err(
			errnie.Validation, "unable to compute bivariate moment",
			BivariateMomentError(BivariateMomentErrorWeightLengthMismatch),
		)

		return bivariateMoment.output
	}

	bivariateMoment.output = core.Scalar[T](T(
		stat.BivariateMoment(
			float64(bivariateMoment.r), float64(bivariateMoment.s),
			xValues, yValues, weights,
		),
	))

	return bivariateMoment.output
}

/*
Reset clears derived state.
*/
func (bivariateMoment *BivariateMoment[T]) Reset() error {
	bivariateMoment.weights = nil
	bivariateMoment.output = core.Scalar[T](0)

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

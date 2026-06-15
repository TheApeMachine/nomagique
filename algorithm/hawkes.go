package algorithm

import (
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/hawkes"
	"github.com/theapemachine/nomagique/statistic"
)

/*
Hawkes validates bivariate exponential-kernel parameters through empirical moments
composed from statistic.BivariateMoment dynamics.
*/
type Hawkes[T ~float64] struct {
	params  hawkes.BivariateParams
	x       []float64
	y       []float64
	weights []float64
	fit     *statistic.BivariateMoment[T]
	cross21 *statistic.BivariateMoment[T]
	cross12 *statistic.BivariateMoment[T]
	output  core.Scalar[T]
}

/*
NewHawkes creates a Hawkes dynamic over configured x and y streams.
r and s select the mixed moment used by Observe for fit diagnostics.
*/
func NewHawkes[T ~float64](
	params hawkes.BivariateParams,
	r, s float64,
	x, y, weights []float64,
) *Hawkes[T] {
	return &Hawkes[T]{
		params:  params,
		x:       x,
		y:       y,
		weights: weights,
		fit:     statistic.NewBivariateMoment[T](T(r), T(s), x, y, weights),
		cross21: statistic.NewBivariateMoment[T](2, 1, x, y, weights),
		cross12: statistic.NewBivariateMoment[T](1, 2, x, y, weights),
	}
}

/*
Observe returns moment-fit confidence for the configured moment and parameters.
*/
func (hawkesProcess *Hawkes[T]) Observe(_ ...core.Number[T]) core.Scalar[T] {
	empirical := float64(hawkesProcess.fit.Observe())
	momentR, momentS := hawkesProcess.fit.Powers()

	theoretical, ok := hawkes.TheoreticalCentralMoment(
		hawkesProcess.params, float64(momentR), float64(momentS),
	)

	if !ok {
		return hawkesProcess.output
	}

	hawkesProcess.output = core.Scalar[T](T(
		hawkes.MomentConfidence(empirical, theoretical),
	))

	return hawkesProcess.output
}

/*
MethodOfMoments derives stable seed parameters from the configured streams.
*/
func (hawkesProcess *Hawkes[T]) MethodOfMoments() (hawkes.BivariateParams, bool) {
	xValues, yValues, weights, ok := hawkesProcess.samples()

	if !ok {
		return hawkes.BivariateParams{}, false
	}

	return hawkes.MethodOfMoments(xValues, yValues, weights, hawkesProcess.params.Beta)
}

/*
CrossAsymmetry compares third-order mixed moments between the configured streams.
*/
func (hawkesProcess *Hawkes[T]) CrossAsymmetry() core.Scalar[T] {
	moment21 := float64(hawkesProcess.cross21.Observe())
	moment12 := float64(hawkesProcess.cross12.Observe())

	return core.Scalar[T](T(moment21 - moment12))
}

/*
Reset clears derived state.
*/
func (hawkesProcess *Hawkes[T]) Reset() error {
	hawkesProcess.weights = nil
	hawkesProcess.output = core.Scalar[T](0)

	if err := hawkesProcess.fit.Reset(); err != nil {
		return err
	}

	if err := hawkesProcess.cross21.Reset(); err != nil {
		return err
	}

	return hawkesProcess.cross12.Reset()
}

func (hawkesProcess *Hawkes[T]) samples() (xValues, yValues, weights []float64, ok bool) {
	xValues = hawkesProcess.x
	yValues = hawkesProcess.y
	weights = hawkesProcess.weights

	if len(weights) == 0 {
		weights = nil
	}

	ok = len(xValues) == len(yValues) && len(xValues) >= 2

	if len(weights) != 0 && len(weights) != len(xValues) {
		ok = false
	}

	return xValues, yValues, weights, ok
}

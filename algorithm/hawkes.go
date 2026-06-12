package algorithm

import (
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/kernel/hawkes"
	"github.com/theapemachine/nomagique/statistic"
)

/*
Hawkes validates bivariate exponential-kernel parameters through empirical moments
composed from statistic.BivariateMoment dynamics.
*/
type Hawkes struct {
	params  hawkes.BivariateParams
	x       core.Numbers
	y       core.Numbers
	weights core.Numbers
	fit     *statistic.BivariateMoment
	cross21 *statistic.BivariateMoment
	cross12 *statistic.BivariateMoment
}

/*
NewHawkes creates a Hawkes dynamic over configured x and y streams.
r and s select the mixed moment used by Observe for fit diagnostics.
*/
func NewHawkes(
	params hawkes.BivariateParams,
	r, s float64,
	x, y, weights core.Numbers,
) *Hawkes {
	return &Hawkes{
		params:  params,
		x:       x,
		y:       y,
		weights: weights,
		fit:     statistic.NewBivariateMoment(r, s, x, y, weights),
		cross21: statistic.NewBivariateMoment(2, 1, x, y, weights),
		cross12: statistic.NewBivariateMoment(1, 2, x, y, weights),
	}
}

/*
Observe returns moment-fit confidence for the configured moment and parameters.
*/
func (hawkesProcess *Hawkes) Observe(_ ...core.Number) core.Float64 {
	empirical := float64(hawkesProcess.fit.Observe())
	momentR, momentS := hawkesProcess.fit.Powers()

	theoretical, ok := hawkes.TheoreticalCentralMoment(
		hawkesProcess.params, momentR, momentS,
	)

	if !ok {
		return 0
	}

	return core.Float64(hawkes.MomentConfidence(empirical, theoretical))
}

/*
MethodOfMoments derives stable seed parameters from the configured streams.
*/
func (hawkesProcess *Hawkes) MethodOfMoments() (hawkes.BivariateParams, bool) {
	xValues, yValues, weights, ok := hawkesProcess.samples()

	if !ok {
		return hawkes.BivariateParams{}, false
	}

	return hawkes.MethodOfMoments(xValues, yValues, weights, hawkesProcess.params.Beta)
}

/*
CrossAsymmetry compares third-order mixed moments between the configured streams.
*/
func (hawkesProcess *Hawkes) CrossAsymmetry() core.Float64 {
	moment21 := float64(hawkesProcess.cross21.Observe())
	moment12 := float64(hawkesProcess.cross12.Observe())

	return core.Float64(moment21 - moment12)
}

/*
Reset clears derived state.
*/
func (hawkesProcess *Hawkes) Reset() error {
	hawkesProcess.weights = nil

	if err := hawkesProcess.fit.Reset(); err != nil {
		return err
	}

	if err := hawkesProcess.cross21.Reset(); err != nil {
		return err
	}

	return hawkesProcess.cross12.Reset()
}

func (hawkesProcess *Hawkes) samples() (xValues, yValues, weights []float64, ok bool) {
	xValues = nomagique.Samples(hawkesProcess.x)
	yValues = nomagique.Samples(hawkesProcess.y)
	weights = hawkesProcess.weights.Float64()

	if len(weights) == 0 {
		weights = nil
	}

	ok = len(xValues) == len(yValues) && len(xValues) >= 2

	if len(weights) != 0 && len(weights) != len(xValues) {
		ok = false
	}

	return xValues, yValues, weights, ok
}

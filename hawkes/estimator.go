package hawkes

import (
	"math"
	"time"
)

/*
BivariateEstimator fits joint Hawkes parameters from an arrival stream.
*/
type BivariateEstimator struct {
	prior BivariateFit
}

/*
NewBivariateEstimator constructs an estimator with an optional warm-start prior.
*/
func NewBivariateEstimator(prior BivariateFit) *BivariateEstimator {
	return &BivariateEstimator{prior: prior}
}

/*
Fit estimates parameters via multi-start L-BFGS on the exact log-likelihood.
*/
func (estimator *BivariateEstimator) Fit(
	stream ArrivalStream,
	horizon time.Time,
) BivariateFit {
	context, ok := NewFitContext(stream, horizon)

	if !ok || !context.EnoughEvents(stream) {
		return BivariateFit{}
	}

	best := BivariateFit{}
	bestLL := math.Inf(-1)
	poisson := context.PoissonFit().WithIntensitiesAt(stream, horizon)

	if poisson.Valid() {
		best = poisson
		bestLL = poisson.LogLikelihood(stream, horizon)
	}

	for _, seed := range estimator.multiStartSeeds(context) {
		candidate := estimator.maximizeLikelihood(stream, horizon, context, seed)

		if candidate.MuX <= 0 {
			continue
		}

		if !estimator.crossLikelihoodValid(stream, horizon, candidate) {
			candidate = candidate.withCrossZeroed().WithIntensitiesAt(stream, horizon)
		}

		logLikelihood := candidate.LogLikelihood(stream, horizon)

		if !estimator.preferCandidate(best, candidate, bestLL, logLikelihood) {
			continue
		}

		bestLL = logLikelihood
		best = candidate
	}

	return best
}

func (estimator *BivariateEstimator) crossLikelihoodValid(
	stream ArrivalStream,
	horizon time.Time,
	fit BivariateFit,
) bool {
	if fit.AlphaXY <= 0 && fit.AlphaYX <= 0 {
		return true
	}

	restricted := BivariateFit{
		MuX:     fit.MuX,
		MuY:     fit.MuY,
		AlphaXX: fit.AlphaXX,
		AlphaYY: fit.AlphaYY,
		Beta:    fit.Beta,
	}

	fitLL := fit.LogLikelihood(stream, horizon)
	restrictedLL := restricted.LogLikelihood(stream, horizon)

	return fitLL+logLikelihoodTolerance(fitLL, restrictedLL) >= restrictedLL
}

func (estimator *BivariateEstimator) preferCandidate(
	current, candidate BivariateFit,
	currentLL, candidateLL float64,
) bool {
	if candidate.MuX <= 0 {
		return false
	}

	if current.MuX <= 0 {
		return true
	}

	improvementTolerance := logLikelihoodTolerance(currentLL, candidateLL)

	if candidateLL > currentLL+improvementTolerance {
		return true
	}

	if math.Abs(candidateLL-currentLL) > math.Sqrt(improvementTolerance) {
		return false
	}

	crossCurrent := current.AlphaXY + current.AlphaYX
	crossCandidate := candidate.AlphaXY + candidate.AlphaYX

	return crossCandidate > crossCurrent
}

func logLikelihoodTolerance(values ...float64) float64 {
	scale := 1.0

	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}

		absValue := math.Abs(value)

		if absValue > scale {
			scale = absValue
		}
	}

	return math.Sqrt(math.Nextafter(1, 2)-1) * scale
}

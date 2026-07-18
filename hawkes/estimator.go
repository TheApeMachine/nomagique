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
fitRestriction selects the nested statistical model optimized against the same
observation window and data-derived parameter bounds.
*/
type fitRestriction int

const (
	fitUnrestricted fitRestriction = iota
	fitSelfOnly
)

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
	return estimator.fit(stream, horizon, fitUnrestricted)
}

/*
FitSelfOnly re-estimates the baseline, decay, and diagonal excitation while
constraining both cross-excitation terms to zero. It provides the correct
restricted likelihood reference for testing whether cross excitation adds
explanatory power.
*/
func (estimator *BivariateEstimator) FitSelfOnly(
	stream ArrivalStream,
	horizon time.Time,
) BivariateFit {
	return estimator.fit(stream, horizon, fitSelfOnly)
}

func (estimator *BivariateEstimator) fit(
	stream ArrivalStream,
	horizon time.Time,
	restriction fitRestriction,
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
		candidate := estimator.maximizeLikelihoodRestricted(
			stream,
			horizon,
			context,
			seed,
			restriction,
		)

		if candidate.MuX <= 0 {
			continue
		}

		if restriction == fitUnrestricted &&
			!estimator.crossLikelihoodValid(stream, horizon, candidate) {
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

	return candidateLL > currentLL+improvementTolerance
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

	radicand := math.Nextafter(1, 2) - 1
	radicandScale := math.Max(1, math.Abs(radicand))
	tolerance := 32 * radicand * radicandScale

	if radicand < -tolerance {
		panic("hawkes: machine-epsilon radicand is negative beyond tolerance")
	}

	return math.Sqrt(math.Max(0, radicand)) * scale
}

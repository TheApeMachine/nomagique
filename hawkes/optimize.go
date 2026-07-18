package hawkes

import (
	"math"
	"time"

	"github.com/theapemachine/nomagique/decay"
	"gonum.org/v1/gonum/optimize"
)

const (
	lbfgsMemory          = 12
	lbfgsMajorIterations = 80
)

func (estimator *BivariateEstimator) maximizeLikelihoodRestricted(
	stream ArrivalStream,
	horizon time.Time,
	context FitContext,
	start [bivariateParamCount]float64,
	restriction fitRestriction,
) BivariateFit {
	bounds, err := context.logParamBounds()

	if err != nil {
		panic(err)
	}

	freeStart := bounds.encode(start)
	problem := optimize.Problem{
		Func: func(free []float64) float64 {
			value, _, ok := estimator.negLogLikelihoodRestricted(
				free, bounds, stream, horizon, context, restriction,
			)

			if !ok {
				return math.Inf(1)
			}

			return value
		},
		Grad: func(grad, free []float64) {
			_, naturalGrad, ok := estimator.negLogLikelihoodGradRestricted(
				free, bounds, stream, horizon, context, restriction,
			)

			if !ok {
				for index := range grad {
					grad[index] = 0
				}

				return
			}

			jacobian := bounds.softplusJacobian(free)

			for index := range grad {
				grad[index] = naturalGrad[index] * jacobian[index]
			}
		},
	}
	result, err := optimize.Minimize(
		problem,
		freeStart,
		&optimize.Settings{
			GradientThreshold: 1e-6,
			MajorIterations:   lbfgsMajorIterations,
		},
		&optimize.LBFGS{Store: lbfgsMemory},
	)

	if err != nil || result.Status != optimize.Success && result.Status != optimize.GradientThreshold {
		return BivariateFit{}
	}

	fit := fitFromRestrictedLogParams(bounds.decode(result.X), context, restriction)

	if fit.MuX <= 0 {
		return BivariateFit{}
	}

	return fit.WithIntensitiesAt(stream, horizon)
}

func (estimator *BivariateEstimator) negLogLikelihoodRestricted(
	free []float64,
	bounds logParamBounds,
	stream ArrivalStream,
	horizon time.Time,
	context FitContext,
	restriction fitRestriction,
) (float64, BivariateFit, bool) {
	fit := fitFromRestrictedLogParams(bounds.decode(free), context, restriction)

	if fit.MuX <= 0 {
		return math.Inf(1), BivariateFit{}, false
	}

	logLikelihood, _, ok := fit.LogLikelihoodGradient(stream, horizon)

	if !ok {
		return math.Inf(1), BivariateFit{}, false
	}

	return -logLikelihood, fit, true
}

func (estimator *BivariateEstimator) negLogLikelihoodGradRestricted(
	free []float64,
	bounds logParamBounds,
	stream ArrivalStream,
	horizon time.Time,
	context FitContext,
	restriction fitRestriction,
) (float64, [bivariateParamCount]float64, bool) {
	fit := fitFromRestrictedLogParams(bounds.decode(free), context, restriction)

	if fit.MuX <= 0 {
		return math.Inf(1), [bivariateParamCount]float64{}, false
	}

	logLikelihood, naturalGradient, ok := fit.LogLikelihoodGradient(stream, horizon)

	if !ok {
		return math.Inf(1), [bivariateParamCount]float64{}, false
	}

	logGrad := logSpaceGradient(naturalGradient, fit)
	negGrad := [bivariateParamCount]float64{}

	for index := range logGrad {
		negGrad[index] = -logGrad[index]
	}

	return -logLikelihood, negGrad, true
}

func (estimator *BivariateEstimator) multiStartSeeds(
	context FitContext,
) [][bivariateParamCount]float64 {
	muXStart := context.MuXStart()
	muYStart := context.MuYStart()
	betaStart := 1 / context.MedianGapSec
	selfBranchSeed := math.Max(context.BranchFloor, selfBranchShareFromContext(context)*context.BranchCeiling)
	crossBranchSeed, err := crossBranchFloorFromContext(context)

	if err != nil {
		panic(err)
	}

	baseLog := [bivariateParamCount]float64{
		decay.LogPositive(muXStart),
		decay.LogPositive(muYStart),
		decay.LogPositive(betaStart),
		decay.LogPositive(selfBranchSeed),
		decay.LogPositive(crossBranchSeed),
		decay.LogPositive(crossBranchSeed),
		decay.LogPositive(selfBranchSeed),
	}
	seeds := make([][bivariateParamCount]float64, 0, len(context.LocalScales)+2)

	if estimator.prior.Valid() {
		if priorSeed, ok := logParamsFromFit(estimator.prior); ok {
			seeds = append(seeds, priorSeed)
		}
	}

	seeds = append(seeds, baseLog)

	for _, scale := range context.LocalScales {
		perturbed := baseLog
		perturbed[3] += math.Log(scale)
		perturbed[4] += math.Log(scale)
		perturbed[5] += math.Log(scale)
		perturbed[6] += math.Log(scale)
		seeds = append(seeds, perturbed)
	}

	return seeds
}

func logParamsFromFit(fit BivariateFit) ([bivariateParamCount]float64, bool) {
	beta := fit.Beta

	if beta <= 0 {
		return [bivariateParamCount]float64{}, false
	}

	return [bivariateParamCount]float64{
		decay.LogPositive(fit.MuX),
		decay.LogPositive(fit.MuY),
		decay.LogPositive(fit.Beta),
		decay.LogPositive(fit.AlphaXX / beta),
		decay.LogPositive(fit.AlphaXY / beta),
		decay.LogPositive(fit.AlphaYX / beta),
		decay.LogPositive(fit.AlphaYY / beta),
	}, true
}

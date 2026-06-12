package hawkes

import (
	"math"
	"time"

	"gonum.org/v1/gonum/optimize"
)

const (
	lbfgsMemory          = 12
	lbfgsMajorIterations = 80
	softplusLinearAt     = 20
	softplusFloor        = 1e-12
	paramRatioFloor      = 1e-9
)

type logParamBounds struct {
	lower [bivariateParamCount]float64
	upper [bivariateParamCount]float64
}

func (context FitContext) logParamBounds() logParamBounds {
	betaMin := context.BetaCandidates[0]
	betaMax := context.BetaCandidates[len(context.BetaCandidates)-1]
	selfMax := context.BranchCeiling * selfBranchShareFromContext(context)
	crossMax := context.BranchCeiling
	minRate := 1 / context.SpanSec
	maxRate := float64(context.TotalEvents) / context.SpanSec

	return logParamBounds{
		lower: [bivariateParamCount]float64{
			LogPositive(minRate),
			LogPositive(minRate),
			math.Log(betaMin),
			LogPositive(context.BranchFloor),
			LogPositive(1e-9),
			LogPositive(1e-9),
			LogPositive(context.BranchFloor),
		},
		upper: [bivariateParamCount]float64{
			LogPositive(maxRate),
			LogPositive(maxRate),
			math.Log(betaMax),
			math.Log(selfMax),
			math.Log(crossMax),
			math.Log(crossMax),
			math.Log(selfMax),
		},
	}
}

func selfBranchShareFromContext(context FitContext) float64 {
	tune := arrivalTune{
		totalEvents: context.TotalEvents,
		eventsX:     context.EventsX,
		eventsY:     context.EventsY,
	}

	return tune.selfBranchShare()
}

func (bounds logParamBounds) decode(free []float64) [bivariateParamCount]float64 {
	params := [bivariateParamCount]float64{}

	for index := range free {
		span := bounds.upper[index] - bounds.lower[index]

		if span <= 0 {
			params[index] = bounds.lower[index]
			continue
		}

		lift := softplus(free[index])
		params[index] = bounds.lower[index] + span*lift/(1+lift)
	}

	return params
}

func (bounds logParamBounds) encode(params [bivariateParamCount]float64) []float64 {
	free := make([]float64, bivariateParamCount)

	for index := range params {
		span := bounds.upper[index] - bounds.lower[index]

		if span <= 0 {
			free[index] = 0
			continue
		}

		ratio := (params[index] - bounds.lower[index]) / span
		ratio = math.Max(paramRatioFloor, math.Min(1-paramRatioFloor, ratio))
		free[index] = inverseSoftplus(ratio / (1 - ratio))
	}

	return free
}

func (bounds logParamBounds) softplusJacobian(free []float64) [bivariateParamCount]float64 {
	jacobian := [bivariateParamCount]float64{}

	for index := range free {
		span := bounds.upper[index] - bounds.lower[index]

		if span <= 0 {
			continue
		}

		lift := softplus(free[index])
		jacobian[index] = span * softplusDerivative(free[index]) / ((1 + lift) * (1 + lift))
	}

	return jacobian
}

func softplus(value float64) float64 {
	if value > softplusLinearAt {
		return value
	}

	return math.Log1p(math.Exp(value))
}

func inverseSoftplus(value float64) float64 {
	if value > softplusLinearAt {
		return value
	}

	if value <= softplusFloor {
		value = softplusFloor
	}

	return math.Log(math.Expm1(value))
}

func softplusDerivative(value float64) float64 {
	if value > softplusLinearAt {
		return 1
	}

	if value < -softplusLinearAt {
		return math.Exp(value)
	}

	return 1 / (1 + math.Exp(-value))
}

func fitFromLogParams(
	logParams [bivariateParamCount]float64,
	context FitContext,
) BivariateFit {
	muX := math.Exp(logParams[0])
	muY := math.Exp(logParams[1])
	beta := math.Exp(logParams[2])
	branchXX := math.Exp(logParams[3])
	branchXY := math.Exp(logParams[4])
	branchYX := math.Exp(logParams[5])
	branchYY := math.Exp(logParams[6])

	if branchXX > context.BranchCeiling || branchYY > context.BranchCeiling {
		return BivariateFit{}
	}

	crossCap := context.CrossBranchCap(math.Max(branchXX, branchYY))

	if branchXY > crossCap || branchYX > crossCap {
		return BivariateFit{}
	}

	fit := BivariateFit{
		MuX:     muX,
		MuY:     muY,
		AlphaXX: branchXX * beta,
		AlphaXY: branchXY * beta,
		AlphaYX: branchYX * beta,
		AlphaYY: branchYY * beta,
		Beta:    beta,
	}
	fit.SpectralRadius = fit.computeSpectralRadius()

	if fit.SpectralRadius <= context.BranchFloor || fit.SpectralRadius >= criticalBranch {
		return BivariateFit{}
	}

	return fit
}

func (estimator *BivariateEstimator) maximizeLikelihood(
	stream ArrivalStream,
	horizon time.Time,
	context FitContext,
	start [bivariateParamCount]float64,
) BivariateFit {
	bounds := context.logParamBounds()
	freeStart := bounds.encode(start)
	problem := optimize.Problem{
		Func: func(free []float64) float64 {
			value, _, ok := estimator.negLogLikelihood(free, bounds, stream, horizon, context)

			if !ok {
				return math.Inf(1)
			}

			return value
		},
		Grad: func(grad, free []float64) {
			_, naturalGrad, ok := estimator.negLogLikelihoodGrad(
				free, bounds, stream, horizon, context,
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
	result, _ := optimize.Minimize(
		problem,
		freeStart,
		&optimize.Settings{
			GradientThreshold: 1e-6,
			MajorIterations:   lbfgsMajorIterations,
		},
		&optimize.LBFGS{Store: lbfgsMemory},
	)

	fit := fitFromLogParams(bounds.decode(result.X), context)

	if fit.MuX <= 0 {
		return BivariateFit{}
	}

	return fit.WithIntensitiesAt(stream, horizon)
}

func (estimator *BivariateEstimator) negLogLikelihood(
	free []float64,
	bounds logParamBounds,
	stream ArrivalStream,
	horizon time.Time,
	context FitContext,
) (float64, BivariateFit, bool) {
	fit := fitFromLogParams(bounds.decode(free), context)

	if fit.MuX <= 0 {
		return math.Inf(1), BivariateFit{}, false
	}

	logLikelihood, _, ok := fit.LogLikelihoodGradient(stream, horizon)

	if !ok {
		return math.Inf(1), BivariateFit{}, false
	}

	return -logLikelihood, fit, true
}

func (estimator *BivariateEstimator) negLogLikelihoodGrad(
	free []float64,
	bounds logParamBounds,
	stream ArrivalStream,
	horizon time.Time,
	context FitContext,
) (float64, [bivariateParamCount]float64, bool) {
	fit := fitFromLogParams(bounds.decode(free), context)

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
	baseLog := [bivariateParamCount]float64{
		LogPositive(muXStart),
		LogPositive(muYStart),
		LogPositive(betaStart),
		LogPositive(0.2),
		LogPositive(0.05),
		LogPositive(0.05),
		LogPositive(0.2),
	}
	seeds := make([][bivariateParamCount]float64, 0, len(context.LocalScales)+2)

	if estimator.prior.Valid() {
		seeds = append(seeds, logParamsFromFit(estimator.prior))
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

func logParamsFromFit(fit BivariateFit) [bivariateParamCount]float64 {
	beta := fit.Beta

	if beta <= 0 {
		beta = 1
	}

	return [bivariateParamCount]float64{
		LogPositive(fit.MuX),
		LogPositive(fit.MuY),
		LogPositive(fit.Beta),
		LogPositive(fit.AlphaXX / beta),
		LogPositive(fit.AlphaXY / beta),
		LogPositive(fit.AlphaYX / beta),
		LogPositive(fit.AlphaYY / beta),
	}
}

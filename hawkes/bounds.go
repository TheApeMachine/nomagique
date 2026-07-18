package hawkes

import (
	"math"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/decay"
)

const (
	softplusLinearAt = 20
	paramRatioFloor  = 1e-9
)

/*
logParamBounds owns the reversible transform between unconstrained optimizer
coordinates and data-derived parameter bounds. Keeping this responsibility
separate makes the likelihood optimizer operate only on valid model domains.
*/
type logParamBounds struct {
	lower [bivariateParamCount]float64
	upper [bivariateParamCount]float64
}

func (context FitContext) logParamBounds() (logParamBounds, error) {
	betaMin := context.BetaCandidates[0]
	betaMax := context.BetaCandidates[len(context.BetaCandidates)-1]
	selfMax := context.BranchCeiling * selfBranchShareFromContext(context)
	crossMax := context.BranchCeiling
	crossMin, err := crossBranchFloorFromContext(context)

	if err != nil {
		return logParamBounds{}, err
	}

	if !(context.SpanSec > 0) {
		return logParamBounds{}, errnie.Err(
			errnie.Validation,
			"hawkes: log param bounds require positive span",
			nil,
		)
	}

	minRate := 1 / context.SpanSec
	maxRate := float64(context.TotalEvents) / context.SpanSec

	return logParamBounds{
		lower: [bivariateParamCount]float64{
			decay.LogPositive(minRate),
			decay.LogPositive(minRate),
			math.Log(betaMin),
			decay.LogPositive(context.BranchFloor),
			decay.LogPositive(crossMin),
			decay.LogPositive(crossMin),
			decay.LogPositive(context.BranchFloor),
		},
		upper: [bivariateParamCount]float64{
			decay.LogPositive(maxRate),
			decay.LogPositive(maxRate),
			math.Log(betaMax),
			math.Log(selfMax),
			math.Log(crossMax),
			math.Log(crossMax),
			math.Log(selfMax),
		},
	}, nil
}

func (bounds logParamBounds) decode(
	free []float64,
) [bivariateParamCount]float64 {
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

func (bounds logParamBounds) encode(
	params [bivariateParamCount]float64,
) []float64 {
	free := make([]float64, bivariateParamCount)

	for index := range params {
		span := bounds.upper[index] - bounds.lower[index]

		if span <= 0 {
			continue
		}

		ratio := (params[index] - bounds.lower[index]) / span
		ratio = math.Max(paramRatioFloor, math.Min(1-paramRatioFloor, ratio))
		free[index] = inverseSoftplus(ratio / (1 - ratio))
	}

	return free
}

func (bounds logParamBounds) softplusJacobian(
	free []float64,
) [bivariateParamCount]float64 {
	jacobian := [bivariateParamCount]float64{}

	for index := range free {
		span := bounds.upper[index] - bounds.lower[index]

		if span <= 0 {
			continue
		}

		lift := softplus(free[index])
		jacobian[index] = span * softplusDerivative(free[index]) /
			((1 + lift) * (1 + lift))
	}

	return jacobian
}

func crossBranchFloorFromContext(context FitContext) (float64, error) {
	radicand := math.Nextafter(1, 2) - 1
	scale := math.Max(1, math.Abs(radicand))
	tolerance := 32 * radicand * scale

	if radicand < -tolerance {
		return 0, errnie.Err(
			errnie.Validation,
			"hawkes: machine-epsilon radicand is negative beyond tolerance",
			nil,
		)
	}

	machineSqrt := math.Sqrt(math.Max(0, radicand))

	if context.BranchFloor > 0 {
		return context.BranchFloor * machineSqrt, nil
	}

	if !(context.SpanSec > 0) || context.TotalEvents <= 0 {
		return 0, errnie.Err(
			errnie.Validation,
			"hawkes: cross-branch floor requires positive span and event mass",
			nil,
		)
	}

	return 1 / context.SpanSec / float64(context.TotalEvents), nil
}

func selfBranchShareFromContext(context FitContext) float64 {
	return arrivalTune{
		totalEvents: context.TotalEvents,
		eventsX:     context.EventsX,
		eventsY:     context.EventsY,
	}.selfBranchShare()
}

func softplus(value float64) float64 {
	if value > softplusLinearAt {
		return value
	}

	argument := math.Exp(value)

	if !(argument > -1) {
		panic(errnie.Err(
			errnie.Validation,
			"hawkes: softplus Log1p argument must be greater than -1",
			nil,
		))
	}

	return math.Log1p(argument)
}

func inverseSoftplus(value float64) float64 {
	if value > softplusLinearAt {
		return value
	}

	if !(value > 0) {
		panic(errnie.Err(
			errnie.Validation,
			"hawkes: inverseSoftplus argument must be strictly positive",
			nil,
		))
	}

	expm1 := math.Expm1(value)

	if !(expm1 > 0) {
		panic(errnie.Err(
			errnie.Validation,
			"hawkes: inverseSoftplus log argument must be strictly positive",
			nil,
		))
	}

	return math.Log(expm1)
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

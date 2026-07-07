package algorithm

import (
	"math"

	"github.com/theapemachine/errnie"
)

func bookflowLevelTick(level BookLevel) (int64, error) {
	if level.Ticks <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"bookflow-sample: positive integer price tick required",
			nil,
		))
	}

	if level.Price <= 0 || math.IsNaN(level.Price) || math.IsInf(level.Price, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"bookflow-sample: level price projection must be positive and finite",
			nil,
		))
	}

	return level.Ticks, nil
}

func bookflowTickPrice(tick int64, tickSize float64) float64 {
	return float64(tick) * tickSize
}

func bookflowDecayRate(mid, spread float64) float64 {
	if mid <= 0 {
		return 1
	}

	relativeSpread := spread / mid

	if relativeSpread <= 0 {
		return 1
	}

	return 1 / relativeSpread
}

func bookflowToxicPenalty(touchCancel, frameAdd, touchDepth float64) float64 {
	if touchCancel <= 0 || frameAdd <= 0 {
		return 0
	}

	churn := touchCancel / frameAdd

	if touchDepth <= 0 {
		return math.Min(1, churn)
	}

	return math.Min(1, churn*(touchCancel/touchDepth))
}

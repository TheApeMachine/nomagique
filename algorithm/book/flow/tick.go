package flow

import (
	"math"

	"github.com/theapemachine/errnie"
)

func LevelTick(level BookLevel) (int64, error) {
	if level.Ticks <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"-sample: positive integer price tick required",
			nil,
		))
	}

	if level.Price <= 0 || math.IsNaN(level.Price) || math.IsInf(level.Price, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"-sample: level price projection must be positive and finite",
			nil,
		))
	}

	return level.Ticks, nil
}

func TickPrice(tick int64, tickSize float64) float64 {
	return float64(tick) * tickSize
}

func DecayRate(mid, spread float64) float64 {
	if mid <= 0 {
		return 1
	}

	relativeSpread := spread / mid

	if relativeSpread <= 0 {
		return 1
	}

	return 1 / relativeSpread
}

/*
ToxicPenalty measures the share of recently cancelled touch liquidity that was
not replaced on the same side of the atomic book update. Repricing an intact
side therefore contributes no false toxicity, while an actual withdrawal is
scaled against the liquidity still available at the touch.
*/
func ToxicPenalty(touchCancel, frameAdd, touchDepth float64) float64 {
	unreplaced := math.Max(0, touchCancel-frameAdd)

	if unreplaced == 0 {
		return 0
	}

	if touchDepth <= 0 {
		return 1
	}

	return unreplaced / (unreplaced + touchDepth)
}

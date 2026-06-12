package hawkes

import (
	"math"
	"time"
)

const logPositiveFloor = 1e-300

/*
ExpNeg returns exp(-beta * seconds).
*/
func ExpNeg(beta, seconds float64) float64 {
	return math.Exp(-beta * seconds)
}

/*
LogPositive returns log(value) with a positive floor.
*/
func LogPositive(value float64) float64 {
	if value <= logPositiveFloor {
		value = logPositiveFloor
	}

	return math.Log(value)
}

/*
IntensityAt computes Hawkes intensity at horizon from buy and sell timelines.
*/
func IntensityAt(
	buyEvents, sellEvents Timeline,
	horizon time.Time,
	mu, alphaFromBuy, alphaFromSell, beta float64,
) float64 {
	intensity := mu
	intensity += excitationSum(buyEvents, horizon, beta) * alphaFromBuy
	intensity += excitationSum(sellEvents, horizon, beta) * alphaFromSell

	return intensity
}

/*
KernelSupport returns the sum of exp(-beta * remaining) for events before horizon.
*/
func KernelSupport(events Timeline, horizon time.Time, beta float64) float64 {
	return excitationSum(events, horizon, beta)
}

func excitationSum(events Timeline, horizon time.Time, beta float64) float64 {
	sum := 0.0

	for _, eventTime := range events.Times() {
		if eventTime.After(horizon) {
			continue
		}

		remaining := horizon.Sub(eventTime).Seconds()

		if remaining > 0 {
			sum += ExpNeg(beta, remaining)
		}
	}

	return sum
}

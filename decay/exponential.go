package decay

import (
	"math"
	"time"

	"github.com/theapemachine/nomagique/timeline"
)

const logPositiveFloor = 1e-300

/*
ExpNeg returns exp(-beta * age).
*/
func ExpNeg(beta, age float64) float64 {
	return math.Exp(-beta * age)
}

/*
LogPositive returns ln(value), flooring non-positive inputs.
*/
func LogPositive(value float64) float64 {
	if value <= logPositiveFloor {
		value = logPositiveFloor
	}

	return math.Log(value)
}

/*
KernelSupport sums exp(-beta * remaining) over events before horizon.
*/
func KernelSupport(events timeline.Timeline, horizon time.Time, beta float64) float64 {
	return excitationSum(events, horizon, beta)
}

/*
IntensityAt evaluates mu plus alphaOnBuy * sum(buy impulses) plus alphaOnSell * sum(sell impulses).
*/
func IntensityAt(
	buyEvents, sellEvents timeline.Timeline,
	at time.Time,
	mu, alphaOnBuy, alphaOnSell, beta float64,
) float64 {
	intensity := mu
	intensity += excitationSum(buyEvents, at, beta) * alphaOnBuy
	intensity += excitationSum(sellEvents, at, beta) * alphaOnSell

	return intensity
}

func excitationSum(events timeline.Timeline, horizon time.Time, beta float64) float64 {
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

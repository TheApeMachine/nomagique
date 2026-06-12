package prob

import "gonum.org/v1/gonum/stat/distuv"

/*
ObserveBeta ingests a Bernoulli outcome in the unit interval.
*/
func ObserveBeta(state *BetaState, outcome float64) float64 {
	if !state.Ready {
		state.Alpha = 1 + outcome
		state.Beta = 1 + (1 - outcome)
		state.Prev = outcome
		state.Min = outcome
		state.Max = outcome
		state.Ready = true

		return posteriorMean(state)
	}

	return observeBetaReady(state, outcome, outcome, outcome, false)
}

/*
ObserveBetaPair ingests predicted and actual values.
*/
func ObserveBetaPair(state *BetaState, predicted float64, actual float64) float64 {
	success := pairSuccess(predicted, actual)

	if !state.Ready {
		state.Alpha = 1 + success
		state.Beta = 2 - success
		state.Prev = predicted
		state.Min = actual - predicted
		state.Max = actual - predicted
		state.Ready = true

		return posteriorMean(state)
	}

	return observeBetaReady(state, success, predicted, actual, true)
}

func observeBetaReady(
	state *BetaState, success float64, predicted float64, actual float64, fromPair bool,
) float64 {
	tracking := success

	if fromPair {
		tracking = actual - predicted
	}

	if tracking < state.Min {
		state.Min = tracking
	}

	if tracking > state.Max {
		state.Max = tracking
	}

	span := state.Max - state.Min

	if span == 0 {
		state.Prev = predicted
		return posteriorMean(state)
	}

	if fromPair {
		state.Rate = absExact(tracking) / span
	} else {
		delta := absExact(success - state.Prev)
		state.Rate = delta / span
		state.Prev = success
	}

	state.Alpha += state.Rate * success
	state.Beta += state.Rate * (1 - success)

	if fromPair {
		state.Prev = predicted
	}

	return posteriorMean(state)
}

func pairSuccess(predicted float64, actual float64) float64 {
	if actual >= predicted {
		return 1
	}

	return 0
}

func posteriorMean(state *BetaState) float64 {
	if state.Alpha+state.Beta == 0 {
		return 0
	}

	distribution := distuv.Beta{
		Alpha: state.Alpha,
		Beta:  state.Beta,
	}

	return distribution.Mean()
}

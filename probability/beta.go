package probability

import (
	"math"

	"gonum.org/v1/gonum/stat/distuv"
)

/*
BetaState tracks a Beta posterior mean from Bernoulli outcomes.
*/
type BetaState struct {
	Alpha float64
	Beta  float64
	Prev  float64
	Min   float64
	Max   float64
	Rate  float64
	Ready bool
}

/*
Observe ingests one Bernoulli outcome in unit interval and returns posterior mean.
*/
func (state *BetaState) Observe(outcome float64) float64 {
	if !state.Ready {
		state.Alpha = 1 + outcome
		state.Beta = 1 + (1 - outcome)
		state.Prev = outcome
		state.Min = outcome
		state.Max = outcome
		state.Ready = true

		return state.mean()
	}

	return state.observeReady(outcome, outcome, outcome, false)
}

/*
ObservePair ingests predicted and actual values and returns posterior mean.
*/
func (state *BetaState) ObservePair(predicted float64, actual float64) float64 {
	success := 0.0

	if actual >= predicted {
		success = 1
	}

	if !state.Ready {
		state.Alpha = 1 + success
		state.Beta = 1 + (1 - success)
		state.Prev = predicted
		state.Min = actual - predicted
		state.Max = actual - predicted
		state.Ready = true

		return state.mean()
	}

	return state.observeReady(success, predicted, actual, true)
}

/*
ObserveSamples writes one posterior mean per outcome into out.
*/
func (state *BetaState) ObserveSamples(outcomes []float64, out []float64) {
	for index, outcome := range outcomes {
		out[index] = state.Observe(outcome)
	}
}

/*
ObservePairSamples writes one posterior mean per pair into out.
*/
func (state *BetaState) ObservePairSamples(
	predicted []float64, actual []float64, out []float64,
) {
	for index := range predicted {
		out[index] = state.ObservePair(predicted[index], actual[index])
	}
}

/*
Reset clears derived state.
*/
func (state *BetaState) Reset() {
	state.Alpha = 0
	state.Beta = 0
	state.Prev = 0
	state.Min = 0
	state.Max = 0
	state.Rate = 0
	state.Ready = false
}

func (state *BetaState) observeReady(
	success float64, predicted float64, actual float64, fromPair bool,
) float64 {
	tracking := success

	if fromPair {
		tracking = actual - predicted
	}

	state.Min = math.Min(state.Min, tracking)
	state.Max = math.Max(state.Max, tracking)

	span := state.Max - state.Min

	if span == 0 {
		state.Prev = predicted
		return state.mean()
	}

	movement := tracking

	if !fromPair {
		movement = success - state.Prev
	}

	state.Rate = math.Abs(movement) / span
	state.Prev = success

	if fromPair {
		state.Prev = predicted
	}

	state.Alpha += state.Rate * success
	state.Beta += state.Rate * (1 - success)

	return state.mean()
}

func (state *BetaState) mean() float64 {
	if state.Alpha+state.Beta == 0 {
		return 0
	}

	distribution := distuv.Beta{
		Alpha: state.Alpha,
		Beta:  state.Beta,
	}

	return distribution.Mean()
}

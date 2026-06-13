package learning

/*
ObserveWeight ingests a predicted and actual pair and returns adaptive trust.
*/
func ObserveWeight(state *WeightState, predicted float64, actual float64) float64 {
	residual := actual - predicted

	if !state.Ready {
		state.Prev = predicted
		state.Min = residual
		state.Max = residual
		state.Trust = 1
		state.Ready = true
		return state.Trust
	}

	return observeWeightReady(state, predicted, actual, residual)
}

/*
observeWeightReady runs the hot weight path; state must already be Ready.
*/
func observeWeightReady(
	state *WeightState, predicted float64, actual float64, residual float64,
) float64 {
	if residual < state.Min {
		state.Min = residual
	}

	if residual > state.Max {
		state.Max = residual
	}

	span := state.Max - state.Min

	if span == 0 {
		state.Prev = predicted
		return state.Trust
	}

	surprise := absExact(residual) / span
	state.Rate = surprise
	targetTrust := 1 - surprise

	if targetTrust < 0 {
		targetTrust = 0
	}

	state.Trust += surprise * (targetTrust - state.Trust)
	state.Prev = predicted

	return state.Trust
}

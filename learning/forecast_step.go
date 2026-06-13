package learning

/*
ObserveForecast updates scale from settled predicted-vs-actual outcomes.
*/
func ObserveForecast(state *ForecastState, predicted float64, actual float64) float64 {
	if !state.Ready {
		state.Scale = 1
		state.Weight.Ready = false
		_ = ObserveWeight(&state.Weight, predicted, actual)
		state.Ready = true
		return state.Scale
	}

	return observeForecastReady(state, predicted, actual)
}

/*
observeForecastReady runs the hot forecast path; state must already be Ready.
*/
func observeForecastReady(
	state *ForecastState, predicted float64, actual float64,
) float64 {
	trust := ObserveWeight(&state.Weight, predicted, actual)
	surprise := state.Weight.Rate
	learningRate := surprise * (1 - trust)

	if predicted == 0 {
		return state.Scale
	}

	targetScale := actual / predicted
	state.Scale += learningRate * (targetScale - state.Scale)

	return state.Scale
}

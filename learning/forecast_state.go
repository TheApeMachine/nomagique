package learning

/*
ForecastState learns a multiplicative scale from predicted and actual outcomes.
*/
type ForecastState struct {
	Scale  float64
	Weight WeightState
	Ready  bool
}

/*
Observe ingests a predicted and actual pair and returns the current scale.
*/
func (state *ForecastState) Observe(predicted float64, actual float64) float64 {
	return ObserveForecast(state, predicted, actual)
}

/*
ObserveSamples writes one scale value per pair into out.
*/
func (state *ForecastState) ObserveSamples(
	predicted []float64, actual []float64, out []float64,
) {
	observeForecastSamples(state, predicted, actual, out)
}

/*
Reset clears derived state.
*/
func (state *ForecastState) Reset() {
	state.Scale = 0
	state.Weight.Reset()
	state.Ready = false
}

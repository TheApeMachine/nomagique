package learning

/*
WeightState tracks a self-adapting trust weight from prediction error.
*/
type WeightState struct {
	Trust float64
	Prev  float64
	Min   float64
	Max   float64
	Rate  float64
	Ready bool
}

/*
Observe ingests a predicted and actual pair and returns the trust weight.
*/
func (state *WeightState) Observe(predicted float64, actual float64) float64 {
	return ObserveWeight(state, predicted, actual)
}

/*
ObserveSamples writes one trust weight per pair into out.
*/
func (state *WeightState) ObserveSamples(
	predicted []float64, actual []float64, out []float64,
) {
	observeWeightSamples(state, predicted, actual, out)
}

/*
Reset clears derived state.
*/
func (state *WeightState) Reset() {
	state.Trust = 0
	state.Prev = 0
	state.Min = 0
	state.Max = 0
	state.Rate = 0
	state.Ready = false
}

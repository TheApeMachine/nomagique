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

	return observeWeightReady(state, predicted, residual)
}

/*
observeWeightReady runs the hot weight path; state must already be Ready.
*/
func observeWeightReady(
	state *WeightState, predicted float64, residual float64,
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

package adaptive

/*
RangeState tracks the running span of observed samples.
*/
type RangeState struct {
	Min   float64
	Max   float64
	Ready bool
}

/*
Observe ingests one sample and returns the current range span.
*/
func (state *RangeState) Observe(sample float64) float64 {
	return ObserveRange(state, sample)
}

/*
ObserveSamples writes one derived value per sample into out.
*/
func (state *RangeState) ObserveSamples(samples []float64, out []float64) {
	observeRangeSamples(state, samples, out)
}

/*
Reset clears derived state.
*/
func (state *RangeState) Reset() {
	state.Min = 0
	state.Max = 0
	state.Ready = false
}

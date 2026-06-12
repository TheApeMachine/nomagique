package kernel

/*
VarianceState tracks an adaptive mean and variance from the observed sample stream.
*/
type VarianceState struct {
	Mean  float64
	Var   float64
	Prev  float64
	Min   float64
	Max   float64
	Rate  float64
	Ready bool
}

/*
Observe ingests one sample and returns the current variance estimate.
*/
func (state *VarianceState) Observe(sample float64) float64 {
	return ObserveVariance(state, sample)
}

/*
ObserveSamples writes one derived value per sample into out.
*/
func (state *VarianceState) ObserveSamples(samples []float64, out []float64) {
	observeVarianceSamples(state, samples, out)
}

/*
Reset clears derived state.
*/
func (state *VarianceState) Reset() {
	state.Mean = 0
	state.Var = 0
	state.Prev = 0
	state.Min = 0
	state.Max = 0
	state.Rate = 0
	state.Ready = false
}

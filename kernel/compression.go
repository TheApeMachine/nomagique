package kernel

/*
CompressionState scores how far below the running baseline the current sample sits.
*/
type CompressionState struct {
	Baseline float64
	Ready    bool
}

/*
Observe ingests one sample and returns the compression score.
*/
func (state *CompressionState) Observe(sample float64) float64 {
	return ObserveCompression(state, sample)
}

/*
ObserveSamples writes one derived value per sample into out.
*/
func (state *CompressionState) ObserveSamples(samples []float64, out []float64) {
	observeCompressionSamples(state, samples, out)
}

/*
Reset clears derived state.
*/
func (state *CompressionState) Reset() {
	state.Baseline = 0
	state.Ready = false
}

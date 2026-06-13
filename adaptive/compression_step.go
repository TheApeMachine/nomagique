package adaptive

/*
ObserveCompression returns how much lower the sample is versus the running baseline.
*/
func ObserveCompression(state *CompressionState, sample float64) float64 {
	if !state.Ready {
		state.Baseline = sample
		state.Ready = true
		return 0
	}

	return observeCompressionReady(state, sample)
}

/*
observeCompressionReady scores tightening against an established baseline.
*/
func observeCompressionReady(state *CompressionState, sample float64) float64 {
	if sample > state.Baseline {
		state.Baseline = sample
		return 0
	}

	if state.Baseline == 0 {
		return 0
	}

	return (state.Baseline - sample) / state.Baseline
}

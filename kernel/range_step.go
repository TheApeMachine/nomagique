package kernel

/*
ObserveRange ingests one sample into state and returns max minus min.
*/
func ObserveRange(state *RangeState, sample float64) float64 {
	if !state.Ready {
		state.Min = sample
		state.Max = sample
		state.Ready = true
		return 0
	}

	return observeRangeReady(state, sample)
}

/*
observeRangeReady runs the hot range path; state must already be Ready.
*/
func observeRangeReady(state *RangeState, sample float64) float64 {
	if sample < state.Min {
		state.Min = sample
	}

	if sample > state.Max {
		state.Max = sample
	}

	return state.Max - state.Min
}

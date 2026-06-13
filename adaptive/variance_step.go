package adaptive

/*
ObserveVariance ingests one sample into state and returns the variance estimate.
*/
func ObserveVariance(state *VarianceState, sample float64) float64 {
	if !state.Ready {
		state.Mean = sample
		state.Var = 0
		state.Prev = sample
		state.Min = sample
		state.Max = sample
		state.Ready = true
		return 0
	}

	return observeVarianceReady(state, sample)
}

/*
observeVarianceReady runs the hot variance path; state must already be Ready.
*/
func observeVarianceReady(state *VarianceState, sample float64) float64 {
	if sample < state.Min {
		state.Min = sample
	}

	if sample > state.Max {
		state.Max = sample
	}

	span := state.Max - state.Min

	if span == 0 {
		state.Prev = sample
		return state.Var
	}

	delta := absExact(sample - state.Prev)
	state.Rate = delta / span
	deviation := sample - state.Mean
	state.Mean += state.Rate * (sample - state.Mean)
	state.Var += state.Rate * (deviation*deviation - state.Var)
	state.Prev = sample

	return state.Var
}

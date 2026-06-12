package kernel

/*
ObserveMomentum ingests one sample into state and returns signed normalized momentum.
*/
func ObserveMomentum(state *MomentumState, sample float64) float64 {
	if !state.Ready {
		state.Prev = sample
		state.Min = sample
		state.Max = sample
		state.Ready = true
		return 0
	}

	return observeMomentumReady(state, sample)
}

/*
observeMomentumReady runs the hot momentum path; state must already be Ready.
*/
func observeMomentumReady(state *MomentumState, sample float64) float64 {
	if sample < state.Min {
		state.Min = sample
	}

	if sample > state.Max {
		state.Max = sample
	}

	span := state.Max - state.Min

	if span == 0 {
		state.Prev = sample
		return 0
	}

	signed := (sample - state.Prev) / span
	state.Prev = sample

	return signed
}

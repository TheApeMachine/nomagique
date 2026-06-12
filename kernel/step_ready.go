package kernel

/*
observeEMAReady runs the hot EMA path; state must already be Ready.
*/
func observeEMAReady(state *EMAState, sample float64) float64 {
	if sample < state.Min {
		state.Min = sample
	}

	if sample > state.Max {
		state.Max = sample
	}

	span := state.Max - state.Min

	if span == 0 {
		state.Prev = sample
		return state.Value
	}

	delta := absExact(sample - state.Prev)
	state.Rate = delta / span
	state.Value += state.Rate * (sample - state.Value)
	state.Prev = sample

	return state.Value
}

/*
observeDeltaReady runs the hot delta path; state must already be Ready.
*/
func observeDeltaReady(state *DeltaState, sample float64) float64 {
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

	normalized := absExact(sample-state.Prev) / span
	state.Prev = sample

	return normalized
}

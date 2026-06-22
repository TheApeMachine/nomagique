package probability

/*
ObserveCUSUM ingests one sample and returns one-sided cumulative evidence.
*/
func ObserveCUSUM(state *CUSUMState, sample float64) float64 {
	if !state.Ready {
		state.Target = sample
		state.Prev = sample
		state.Min = sample
		state.Max = sample
		state.Positive = 0
		state.Ready = true
		return 0
	}

	return observeCUSUMReady(state, sample)
}

/*
observeCUSUMReady runs the hot CUSUM path; state must already be Ready.
*/
func observeCUSUMReady(state *CUSUMState, sample float64) float64 {
	if sample < state.Min {
		state.Min = sample
	}

	if sample > state.Max {
		state.Max = sample
	}

	span := state.Max - state.Min

	if span == 0 {
		state.Prev = sample
		return state.Positive
	}

	delta := absExact(sample - state.Prev)
	state.Rate = delta / span
	drift := state.Rate * span / 2
	excess := sample - state.Target - drift

	if excess > 0 {
		state.Positive += excess
	}

	if excess < 0 {
		state.Positive = 0
	}

	state.Target += state.Rate * (sample - state.Target)
	state.Prev = sample

	return state.Positive
}

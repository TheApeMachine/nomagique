package adaptive

/*
ObserveEMA ingests one sample into state and returns the derived EMA value.
Single implementation shared by scalar and batch paths.
*/
func ObserveEMA(state *EMAState, sample float64) float64 {
	if !state.Ready {
		state.Value = sample
		state.Prev = sample
		state.Min = sample
		state.Max = sample
		state.Ready = true
		return state.Value
	}

	return observeEMAReady(state, sample)
}

/*
ObserveDelta ingests one sample into state and returns the normalized delta.
*/
func ObserveDelta(state *DeltaState, sample float64) float64 {
	if !state.Ready {
		state.Prev = sample
		state.Min = sample
		state.Max = sample
		state.Ready = true
		return 0
	}

	return observeDeltaReady(state, sample)
}

func absExact(value float64) float64 {
	if value < 0 {
		return -value
	}

	return value
}

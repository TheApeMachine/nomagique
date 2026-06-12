package kernel

/*
ObserveAccumulator integrates sample strength into state.Level.
Positive samples charge, negative samples drain, zero holds.
*/
func ObserveAccumulator(state *AccumulatorState, sample float64) float64 {
	if sample == 0 {
		return state.Level
	}

	state.Level += sample

	return state.Level
}

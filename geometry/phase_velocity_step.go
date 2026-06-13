package geometry

/*
ObservePhaseVelocity returns surprisal mean velocity for the current mean.
*/
func ObservePhaseVelocity(state *PhaseVelocityState, mean float64) float64 {
	if !state.Ready {
		state.Prev = mean
		state.Ready = true

		return 0
	}

	velocity := mean - state.Prev
	state.Prev = mean

	return velocity
}

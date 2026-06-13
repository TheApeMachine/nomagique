package geometry

func observePhaseVelocitySamples(
	state *PhaseVelocityState, means []float64, out []float64,
) {
	for index, mean := range means {
		out[index] = ObservePhaseVelocity(state, mean)
	}
}

package geom

/*
PhaseVelocityState tracks surprisal mean velocity between consecutive samples.
*/
type PhaseVelocityState struct {
	Prev  float64
	Ready bool
}

/*
Observe ingests one mean sample and returns its velocity versus the previous mean.
*/
func (state *PhaseVelocityState) Observe(mean float64) float64 {
	return ObservePhaseVelocity(state, mean)
}

/*
ObserveSamples writes one velocity per sample into out.
*/
func (state *PhaseVelocityState) ObserveSamples(means []float64, out []float64) {
	observePhaseVelocitySamples(state, means, out)
}

/*
Reset clears derived state.
*/
func (state *PhaseVelocityState) Reset() {
	state.Prev = 0
	state.Ready = false
}

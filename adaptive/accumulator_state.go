package adaptive

/*
AccumulatorState integrates signed signal strength into a level with no bounds.
*/
type AccumulatorState struct {
	Level float64
}

/*
Observe ingests one sample and returns the integrated level.
*/
func (state *AccumulatorState) Observe(sample float64) float64 {
	return ObserveAccumulator(state, sample)
}

/*
ObserveSamples writes one derived value per sample into out.
*/
func (state *AccumulatorState) ObserveSamples(samples []float64, out []float64) {
	observeAccumulatorSamples(state, samples, out)
}

/*
Reset clears derived state.
*/
func (state *AccumulatorState) Reset() {
	state.Level = 0
}

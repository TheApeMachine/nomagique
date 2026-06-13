package adaptive

/*
DeltaState tracks a unit-normalized change relative to the running sample range.
*/
type DeltaState struct {
	Prev    float64
	Min     float64
	Max     float64
	Ready   bool
	scratch []float64
}

/*
Observe ingests one sample and returns the normalized delta.
*/
func (state *DeltaState) Observe(sample float64) float64 {
	return ObserveDelta(state, sample)
}

/*
ObserveSamples writes one derived value per sample into out.
*/
func (state *DeltaState) ObserveSamples(samples []float64, out []float64) {
	observeDeltaSamples(state, samples, out)
}

/*
Reset clears derived state.
*/
func (state *DeltaState) Reset() {
	state.Ready = false
	state.Prev = 0
	state.Min = 0
	state.Max = 0
}

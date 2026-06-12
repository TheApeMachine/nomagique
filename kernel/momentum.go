package kernel

/*
MomentumState tracks a signed unit-normalized move relative to the running range.
*/
type MomentumState struct {
	Prev  float64
	Min   float64
	Max   float64
	Ready bool
}

/*
Observe ingests one sample and returns the signed normalized momentum.
*/
func (state *MomentumState) Observe(sample float64) float64 {
	return ObserveMomentum(state, sample)
}

/*
ObserveSamples writes one derived value per sample into out.
*/
func (state *MomentumState) ObserveSamples(samples []float64, out []float64) {
	observeMomentumSamples(state, samples, out)
}

/*
Reset clears derived state.
*/
func (state *MomentumState) Reset() {
	state.Prev = 0
	state.Min = 0
	state.Max = 0
	state.Ready = false
}

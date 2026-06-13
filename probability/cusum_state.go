package probability

/*
CUSUMState accumulates one-sided change evidence from a sample stream.
*/
type CUSUMState struct {
	Target   float64
	Positive float64
	Prev     float64
	Min      float64
	Max      float64
	Rate     float64
	Ready    bool
}

/*
Observe ingests one sample and returns cumulative change evidence.
*/
func (state *CUSUMState) Observe(sample float64) float64 {
	return ObserveCUSUM(state, sample)
}

/*
ObserveSamples writes one evidence value per sample into out.
*/
func (state *CUSUMState) ObserveSamples(samples []float64, out []float64) {
	observeCUSUMSamples(state, samples, out)
}

/*
Reset clears derived state.
*/
func (state *CUSUMState) Reset() {
	state.Target = 0
	state.Positive = 0
	state.Prev = 0
	state.Min = 0
	state.Max = 0
	state.Rate = 0
	state.Ready = false
}

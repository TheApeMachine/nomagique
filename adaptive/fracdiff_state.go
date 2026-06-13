package adaptive

/*
FracDiffState applies a fixed-width fractional differencing filter to recent samples.
*/
type FracDiffState struct {
	Prev    float64
	Min     float64
	Max     float64
	Order   float64
	Ready   bool
	Width   int
	Head    int
	Count   int
	History []float64
	Weights []float64
}

/*
Observe ingests one sample and returns the fractionally differenced value.
*/
func (state *FracDiffState) Observe(sample float64) float64 {
	return ObserveFracDiff(state, sample)
}

/*
ObserveSamples writes one derived value per sample into out.
*/
func (state *FracDiffState) ObserveSamples(samples []float64, out []float64) {
	observeFracDiffSamples(state, samples, out)
}

/*
Reset clears derived state.
*/
func (state *FracDiffState) Reset() {
	state.Prev = 0
	state.Min = 0
	state.Max = 0
	state.Order = 0
	state.Ready = false
	state.Width = 0
	state.Head = 0
	state.Count = 0
	state.History = nil
	state.Weights = nil
}

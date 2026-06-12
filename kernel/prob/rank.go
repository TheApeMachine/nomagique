package prob

/*
RankState tracks the empirical probability that observations fall at or below the current sample.
*/
type RankState struct {
	Prev    float64
	Min     float64
	Max     float64
	Head    int
	Count   int
	History []float64
	Ready   bool
}

/*
Observe ingests one sample and returns the empirical rank probability in the unit interval.
*/
func (state *RankState) Observe(sample float64) float64 {
	return ObserveRank(state, sample)
}

/*
ObserveSamples writes one rank probability per sample into out.
*/
func (state *RankState) ObserveSamples(samples []float64, out []float64) {
	observeRankSamples(state, samples, out)
}

/*
Reset clears derived state.
*/
func (state *RankState) Reset() {
	state.Prev = 0
	state.Min = 0
	state.Max = 0
	state.Head = 0
	state.Count = 0
	state.History = nil
	state.Ready = false
}

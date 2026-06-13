package probability

/*
BetaState tracks a Beta posterior mean from Bernoulli outcomes.
*/
type BetaState struct {
	Alpha float64
	Beta  float64
	Prev  float64
	Min   float64
	Max   float64
	Rate  float64
	Ready bool
}

/*
Observe ingests one Bernoulli outcome in unit interval and returns posterior mean.
*/
func (state *BetaState) Observe(outcome float64) float64 {
	return ObserveBeta(state, outcome)
}

/*
ObservePair ingests predicted and actual values and returns posterior mean.
*/
func (state *BetaState) ObservePair(predicted float64, actual float64) float64 {
	return ObserveBetaPair(state, predicted, actual)
}

/*
ObserveSamples writes one posterior mean per outcome into out.
*/
func (state *BetaState) ObserveSamples(outcomes []float64, out []float64) {
	observeBetaSamples(state, outcomes, out)
}

/*
ObservePairSamples writes one posterior mean per pair into out.
*/
func (state *BetaState) ObservePairSamples(
	predicted []float64, actual []float64, out []float64,
) {
	observeBetaPairSamples(state, predicted, actual, out)
}

/*
Reset clears derived state.
*/
func (state *BetaState) Reset() {
	state.Alpha = 0
	state.Beta = 0
	state.Prev = 0
	state.Min = 0
	state.Max = 0
	state.Rate = 0
	state.Ready = false
}

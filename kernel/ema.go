/*
Package kernel provides exact float64 state machines for adaptive signal dynamics.
Batch paths avoid interface overhead; all arithmetic is IEEE-754 float64 with no approximations.
*/
package kernel

/*
EMAState is a volatility-adaptive exponential moving average in explicit float64 state.
*/
type EMAState struct {
	Value   float64
	Prev    float64
	Min     float64
	Max     float64
	Rate    float64
	Ready   bool
	scratch []float64
}

/*
Observe ingests one sample and returns the derived EMA value.
*/
func (state *EMAState) Observe(sample float64) float64 {
	return ObserveEMA(state, sample)
}

/*
ObserveSamples writes one derived value per sample into out.
len(out) must be >= len(samples). Ready samples use an inlined hot batch driver.
*/
func (state *EMAState) ObserveSamples(samples []float64, out []float64) {
	observeSamples(state, samples, out)
}

/*
Reset clears derived state.
*/
func (state *EMAState) Reset() {
	state.Ready = false
	state.Value = 0
	state.Prev = 0
	state.Min = 0
	state.Max = 0
	state.Rate = 0
}

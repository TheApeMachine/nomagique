package adaptive

import "math"

/*
ZScoreState tracks adaptive scale for a normalized surprise score.
*/
type ZScoreState struct {
	Mean  float64
	Var   float64
	Prev  float64
	Min   float64
	Max   float64
	Rate  float64
	Ready bool
}

/*
Observe ingests one sample and returns the z-score versus internal mean and variance.
*/
func (state *ZScoreState) Observe(sample float64) float64 {
	return ObserveZScore(state, sample, 0, false)
}

/*
ObserveSamples writes one derived value per sample into out.
*/
func (state *ZScoreState) ObserveSamples(samples []float64, out []float64) {
	observeZScoreSamples(state, samples, out)
}

/*
Reset clears derived state.
*/
func (state *ZScoreState) Reset() {
	state.Mean = 0
	state.Var = 0
	state.Prev = 0
	state.Min = 0
	state.Max = 0
	state.Rate = 0
	state.Ready = false
}

func zScoreFromDeviation(deviation float64, variance float64) float64 {
	if variance <= 0 {
		return 0
	}

	return deviation / math.Sqrt(variance)
}

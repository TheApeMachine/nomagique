package learn

/*
SampleRatioState maps predicted and actual pairs into calibration samples.
*/
type SampleRatioState struct {
	Prev      float64
	Min       float64
	Max       float64
	PeakRatio float64
	Ready     bool
}

/*
Observe ingests a predicted and actual pair and returns the calibration sample.
*/
func (state *SampleRatioState) Observe(predicted float64, actual float64) float64 {
	return ObserveSampleRatio(state, predicted, actual)
}

/*
ObserveSamples writes one calibration sample per pair into out.
*/
func (state *SampleRatioState) ObserveSamples(
	predicted []float64, actual []float64, out []float64,
) {
	observeSampleRatioSamples(state, predicted, actual, out)
}

/*
Reset clears derived state.
*/
func (state *SampleRatioState) Reset() {
	state.Prev = 0
	state.Min = 0
	state.Max = 0
	state.PeakRatio = 0
	state.Ready = false
}

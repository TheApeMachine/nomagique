package adaptive

/*
observeSamples ingests samples in order, matching sequential ObserveEMA.
*/
func observeSamples(state *EMAState, samples []float64, out []float64) {
	count := len(samples)
	index := 0

	for index < count && !state.Ready {
		out[index] = ObserveEMA(state, samples[index])
		index++
	}

	if index >= count {
		return
	}

	observeEMASamplesHot(state, samples[index:count], out[index:count])
}

/*
observeDeltaSamples ingests samples in order, matching sequential ObserveDelta.
*/
func observeDeltaSamples(state *DeltaState, samples []float64, out []float64) {
	count := len(samples)
	index := 0

	for index < count && !state.Ready {
		out[index] = ObserveDelta(state, samples[index])
		index++
	}

	if index >= count {
		return
	}

	observeDeltaSamplesHot(state, samples[index:count], out[index:count])
}

/*
observeAccumulatorSamples ingests samples in order, matching sequential ObserveAccumulator.
*/
func observeAccumulatorSamples(state *AccumulatorState, samples []float64, out []float64) {
	for index, sample := range samples {
		out[index] = ObserveAccumulator(state, sample)
	}
}

/*
observeCompressionSamples ingests samples in order, matching sequential ObserveCompression.
*/
func observeCompressionSamples(state *CompressionState, samples []float64, out []float64) {
	for index, sample := range samples {
		out[index] = ObserveCompression(state, sample)
	}
}

/*
observeFracDiffSamples ingests samples in order, matching sequential ObserveFracDiff.
*/
func observeFracDiffSamples(state *FracDiffState, samples []float64, out []float64) {
	for index, sample := range samples {
		out[index] = ObserveFracDiff(state, sample)
	}
}

/*
observeVarianceSamples ingests samples in order, matching sequential ObserveVariance.
*/
func observeVarianceSamples(state *VarianceState, samples []float64, out []float64) {
	for index, sample := range samples {
		out[index] = ObserveVariance(state, sample)
	}
}

/*
observeZScoreSamples ingests samples in order, matching sequential ObserveZScore without anchors.
*/
func observeZScoreSamples(state *ZScoreState, samples []float64, out []float64) {
	for index, sample := range samples {
		out[index] = ObserveZScore(state, sample, 0, false)
	}
}

/*
observeMomentumSamples ingests samples in order, matching sequential ObserveMomentum.
*/
func observeMomentumSamples(state *MomentumState, samples []float64, out []float64) {
	for index, sample := range samples {
		out[index] = ObserveMomentum(state, sample)
	}
}

/*
observeRangeSamples ingests samples in order, matching sequential ObserveRange.
*/
func observeRangeSamples(state *RangeState, samples []float64, out []float64) {
	for index, sample := range samples {
		out[index] = ObserveRange(state, sample)
	}
}

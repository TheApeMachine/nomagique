package learning

/*
ObserveSampleRatio maps one predicted and actual pair to a calibration sample.
*/
func ObserveSampleRatio(
	state *SampleRatioState, predicted float64, actual float64,
) float64 {
	if !state.Ready {
		state.Min = actual - predicted
		state.Max = actual - predicted
		state.Prev = predicted
		state.Ready = true

		return bootstrapSampleRatio(predicted, actual)
	}

	return observeSampleRatioReady(state, predicted, actual)
}

func bootstrapSampleRatio(predicted float64, actual float64) float64 {
	ratio := rawSampleRatio(predicted, actual)
	ceiling := maxSampleRatioCeiling(0, absExact(predicted))

	if ratio > ceiling {
		return ceiling
	}

	return ratio
}

/*
observeSampleRatioReady runs the hot sample-ratio path; state must already be Ready.
*/
func observeSampleRatioReady(
	state *SampleRatioState, predicted float64, actual float64,
) float64 {
	residual := actual - predicted

	if residual < state.Min {
		state.Min = residual
	}

	if residual > state.Max {
		state.Max = residual
	}

	span := state.Max - state.Min
	ratio := rawSampleRatio(predicted, actual)
	ceiling := maxSampleRatioCeiling(span, absExact(state.Prev))

	if ratio > ceiling {
		ratio = ceiling
	}

	if ratio > state.PeakRatio {
		state.PeakRatio = ratio
	}

	state.Prev = predicted

	return ratio
}

func rawSampleRatio(predicted float64, actual float64) float64 {
	if actual >= predicted {
		return actual / predicted
	}

	lossRatio := 1 + actual/predicted

	if lossRatio < 0 {
		return 0
	}

	return lossRatio
}

/*
maxSampleRatioCeiling derives the upper calibration bound from observed spread.
*/
func maxSampleRatioCeiling(span float64, reference float64) float64 {
	if span > 0 {
		return 1 + 1/span
	}

	if reference > 0 {
		return 1 + 1/reference
	}

	return 1
}

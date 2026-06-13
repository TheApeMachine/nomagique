package adaptive

/*
observeEMASamplesHotPhased evaluates the ready EMA in two phases:
vector-friendly prefix min/max, then a fused rate+value recurrence.
Bit-identical to observeEMAReady stepped sample-by-sample.
*/
func observeEMASamplesHotPhased(state *EMAState, samples []float64, out []float64) {
	count := len(samples)
	work := emaBatchScratch(state, count)
	minOut := work[0:count]
	maxOut := work[count : 2*count]

	prefixMinMaxVector(state.Min, state.Max, samples, minOut, maxOut)

	value, rate := applyEMAValuesFused(state.Value, state.Prev, samples, minOut, maxOut, out)

	state.Value = value
	state.Prev = samples[count-1]
	state.Min = minOut[count-1]
	state.Max = maxOut[count-1]

	span := maxOut[count-1] - minOut[count-1]

	if span != 0 {
		state.Rate = rate
	}
}

func applyEMAValuesFused(
	seedValue float64, seedPrev float64,
	samples []float64, minOut []float64, maxOut []float64, out []float64,
) (float64, float64) {
	value := seedValue
	prevSample := seedPrev
	lastRate := 0.0

	for index, sample := range samples {
		span := maxOut[index] - minOut[index]

		if span == 0 {
			out[index] = value
			prevSample = sample
			continue
		}

		delta := sample - prevSample

		if delta < 0 {
			delta = -delta
		}

		lastRate = delta / span
		value += lastRate * (sample - value)
		out[index] = value
		prevSample = sample
	}

	return value, lastRate
}

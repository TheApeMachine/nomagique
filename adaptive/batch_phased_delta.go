package adaptive

/*
observeDeltaSamplesHotPhased evaluates the ready delta via prefix min/max then
per-index normalized moves. Bit-identical to observeDeltaReady per sample.
*/
func observeDeltaSamplesHotPhased(state *DeltaState, samples []float64, out []float64) {
	count := len(samples)
	work := deltaBatchScratch(state, count)
	minOut := work[0:count]
	maxOut := work[count : 2*count]

	prefixMinMaxVector(state.Min, state.Max, samples, minOut, maxOut)
	applyDeltaOutputs(state.Prev, samples, minOut, maxOut, out)

	state.Prev = samples[count-1]
	state.Min = minOut[count-1]
	state.Max = maxOut[count-1]
}

func applyDeltaOutputs(
	seedPrev float64, samples []float64,
	minOut []float64, maxOut []float64, out []float64,
) {
	prevSample := seedPrev

	for index, sample := range samples {
		span := maxOut[index] - minOut[index]

		if span == 0 {
			out[index] = 0
			prevSample = sample
			continue
		}

		delta := sample - prevSample

		if delta < 0 {
			delta = -delta
		}

		out[index] = delta / span
		prevSample = sample
	}
}

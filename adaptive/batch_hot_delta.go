package adaptive

func observeDeltaSamplesHotUnrolled(state *DeltaState, samples []float64, out []float64) {
	count := len(samples)
	index := 0

	for index+7 < count {
		out[index+0] = observeDeltaReady(state, samples[index+0])
		out[index+1] = observeDeltaReady(state, samples[index+1])
		out[index+2] = observeDeltaReady(state, samples[index+2])
		out[index+3] = observeDeltaReady(state, samples[index+3])
		out[index+4] = observeDeltaReady(state, samples[index+4])
		out[index+5] = observeDeltaReady(state, samples[index+5])
		out[index+6] = observeDeltaReady(state, samples[index+6])
		out[index+7] = observeDeltaReady(state, samples[index+7])
		index += 8
	}

	for index < count {
		out[index] = observeDeltaReady(state, samples[index])
		index++
	}
}

package kernel

const batchScratchSlots = 3

func emaBatchScratch(state *EMAState, sampleCount int) []float64 {
	need := sampleCount * batchScratchSlots

	if cap(state.scratch) < need {
		state.scratch = make([]float64, need)
	}

	return state.scratch[:need]
}

func deltaBatchScratch(state *DeltaState, sampleCount int) []float64 {
	need := sampleCount * 2

	if cap(state.scratch) < need {
		state.scratch = make([]float64, need)
	}

	return state.scratch[:need]
}

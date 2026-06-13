//go:build !arm64

package adaptive

func observeEMASamplesHot(state *EMAState, samples []float64, out []float64) {
	observeEMASamplesHotInlined(state, samples, out)
}

func observeDeltaSamplesHot(state *DeltaState, samples []float64, out []float64) {
	observeDeltaSamplesHotUnrolled(state, samples, out)
}

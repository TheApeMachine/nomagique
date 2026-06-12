//go:build arm64

package kernel

type emaSamplesHotParams struct {
	state   *EMAState
	count   int
	samples *float64
	out     *float64
}

//go:noescape
func observeEMASamplesHotARM64(params *emaSamplesHotParams)

func observeEMASamplesHot(state *EMAState, samples []float64, out []float64) {
	count := len(samples)

	if count == 0 {
		return
	}

	observeEMASamplesHotARM64(&emaSamplesHotParams{
		state:   state,
		count:   count,
		samples: &samples[0],
		out:     &out[0],
	})
}

func observeDeltaSamplesHot(state *DeltaState, samples []float64, out []float64) {
	observeDeltaSamplesHotUnrolled(state, samples, out)
}

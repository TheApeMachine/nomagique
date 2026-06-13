package adaptive

import "testing"

func BenchmarkObserveEMASamplesHotInlined(testingTB *testing.B) {
	state := EMAState{}
	_ = ObserveEMA(&state, 0)
	samples := make([]float64, 1024)
	out := make([]float64, len(samples))

	for index := range samples {
		samples[index] = float64(index)
	}

	for testingTB.Loop() {
		state.Reset()
		_ = ObserveEMA(&state, 0)
		observeEMASamplesHotInlined(&state, samples, out)
	}
}

func BenchmarkObserveEMASamplesHotPhased(testingTB *testing.B) {
	state := EMAState{}
	_ = ObserveEMA(&state, 0)
	samples := make([]float64, 1024)
	out := make([]float64, len(samples))

	for index := range samples {
		samples[index] = float64(index)
	}

	for testingTB.Loop() {
		state.Reset()
		_ = ObserveEMA(&state, 0)
		observeEMASamplesHotPhased(&state, samples, out)
	}
}

func BenchmarkObserveEMASamplesHot(testingTB *testing.B) {
	state := EMAState{}
	_ = ObserveEMA(&state, 0)
	samples := make([]float64, 1024)
	out := make([]float64, len(samples))

	for index := range samples {
		samples[index] = float64(index)
	}

	for testingTB.Loop() {
		state.Reset()
		_ = ObserveEMA(&state, 0)
		observeEMASamplesHot(&state, samples, out)
	}
}

func BenchmarkObserveDeltaSamplesHotUnrolled(testingTB *testing.B) {
	state := DeltaState{}
	_ = ObserveDelta(&state, 0)
	samples := make([]float64, 1024)
	out := make([]float64, len(samples))

	for index := range samples {
		samples[index] = float64(index)
	}

	for testingTB.Loop() {
		state.Reset()
		_ = ObserveDelta(&state, 0)
		observeDeltaSamplesHotUnrolled(&state, samples, out)
	}
}

func BenchmarkObserveDeltaSamplesHotPhased(testingTB *testing.B) {
	state := DeltaState{}
	_ = ObserveDelta(&state, 0)
	samples := make([]float64, 1024)
	out := make([]float64, len(samples))

	for index := range samples {
		samples[index] = float64(index)
	}

	for testingTB.Loop() {
		state.Reset()
		_ = ObserveDelta(&state, 0)
		observeDeltaSamplesHotPhased(&state, samples, out)
	}
}

func BenchmarkObserveDeltaSamplesHot(testingTB *testing.B) {
	state := DeltaState{}
	_ = ObserveDelta(&state, 0)
	samples := make([]float64, 1024)
	out := make([]float64, len(samples))

	for index := range samples {
		samples[index] = float64(index)
	}

	for testingTB.Loop() {
		state.Reset()
		_ = ObserveDelta(&state, 0)
		observeDeltaSamplesHot(&state, samples, out)
	}
}

func BenchmarkPrefixMinMax(testingTB *testing.B) {
	samples := make([]float64, 1024)
	minOut := make([]float64, len(samples))
	maxOut := make([]float64, len(samples))

	for index := range samples {
		samples[index] = float64(index) - 512
	}

	for testingTB.Loop() {
		prefixMinMax(-100, 100, samples, minOut, maxOut)
	}
}

func BenchmarkPrefixMinMaxVector(testingTB *testing.B) {
	samples := make([]float64, 1024)
	minOut := make([]float64, len(samples))
	maxOut := make([]float64, len(samples))

	for index := range samples {
		samples[index] = float64(index) - 512
	}

	for testingTB.Loop() {
		prefixMinMaxVector(-100, 100, samples, minOut, maxOut)
	}
}

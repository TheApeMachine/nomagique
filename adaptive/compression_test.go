package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestCompressionState_Observe(testingTB *testing.T) {
	Convey("Given a fresh compression state", testingTB, func() {
		state := CompressionState{}

		Convey("When bootstrapping", func() {
			value := state.Observe(10)

			Convey("It should return zero", func() {
				So(value, ShouldEqual, 0)
			})
		})
	})

	Convey("Given compression history", testingTB, func() {
		state := CompressionState{}
		_ = state.Observe(100)

		Convey("When the sample tightens below baseline", func() {
			value := state.Observe(50)

			Convey("It should return a positive score", func() {
				So(value, ShouldEqual, 0.5)
			})
		})

		Convey("When the sample widens above baseline", func() {
			value := state.Observe(120)

			Convey("It should reset the score and raise baseline", func() {
				So(value, ShouldEqual, 0)
				So(state.Baseline, ShouldEqual, 120)
			})
		})

		Convey("When baseline is unchanged", func() {
			value := state.Observe(100)

			Convey("It should return zero", func() {
				So(value, ShouldEqual, 0)
			})
		})
	})

	Convey("Given a collapsed baseline", testingTB, func() {
		state := CompressionState{}
		_ = state.Observe(0)

		Convey("When observing zero again", func() {
			value := state.Observe(0)

			Convey("It should return zero", func() {
				So(value, ShouldEqual, 0)
			})
		})
	})
}

func TestCompressionState_ObserveSamples(testingTB *testing.T) {
	Convey("Given samples", testingTB, func() {
		state := CompressionState{}
		samples := []float64{100, 50, 120}
		out := make([]float64, len(samples))

		Convey("When observing in batch", func() {
			state.ObserveSamples(samples, out)

			Convey("It should match sequential observation", func() {
				expect := CompressionState{}
				for index, sample := range samples {
					So(out[index], ShouldEqual, expect.Observe(sample))
				}
			})
		})
	})
}

func TestCompressionState_Reset(testingTB *testing.T) {
	Convey("Given compression state", testingTB, func() {
		state := CompressionState{}
		_ = state.Observe(3)

		Convey("When reset", func() {
			state.Reset()

			Convey("It should clear readiness", func() {
				So(state.Ready, ShouldBeFalse)
			})
		})
	})
}

func TestObserveCompression(testingTB *testing.T) {
	Convey("Given ObserveCompression", testingTB, func() {
		state := CompressionState{}

		Convey("It should match method observation", func() {
			So(ObserveCompression(&state, 8), ShouldEqual, state.Observe(8))
		})
	})
}

func TestObserveCompressionReady(testingTB *testing.T) {
	Convey("Given ObserveCompressionReady", testingTB, func() {
		state := CompressionState{Baseline: 20, Ready: true}

		Convey("It should match ready observation", func() {
			So(observeCompressionReady(&state, 10), ShouldEqual, state.Observe(10))
		})
	})
}

func BenchmarkCompressionState_Observe(testingTB *testing.B) {
	state := CompressionState{}
	_ = state.Observe(10)

	for testingTB.Loop() {
		_ = state.Observe(9)
	}
}

func BenchmarkCompressionState_ObserveSamples(testingTB *testing.B) {
	state := CompressionState{}
	samples := make([]float64, 1024)
	out := make([]float64, len(samples))

	for index := range samples {
		samples[index] = float64(index % 20)
	}

	for testingTB.Loop() {
		state.Reset()
		state.ObserveSamples(samples, out)
	}
}

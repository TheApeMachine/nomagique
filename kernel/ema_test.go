package kernel

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestEMAState_Observe(testingTB *testing.T) {
	Convey("Given a fresh EMA state", testingTB, func() {
		state := EMAState{}

		Convey("When bootstrapping", func() {
			value := state.Observe(10)

			Convey("It should adopt the sample exactly", func() {
				So(value, ShouldEqual, 10)
			})
		})
	})

	Convey("Given an EMA with collapsed range", testingTB, func() {
		state := EMAState{}
		_ = state.Observe(8)
		value := state.Observe(8)

		Convey("It should keep the prior value", func() {
			So(value, ShouldEqual, 8)
		})
	})
}

func TestEMAState_ObserveSamples(testingTB *testing.T) {
	Convey("Given samples", testingTB, func() {
		state := EMAState{}
		samples := []float64{10, 5, 20}
		out := make([]float64, len(samples))

		Convey("When observing in batch", func() {
			state.ObserveSamples(samples, out)

			Convey("It should match sequential observation", func() {
				expect := EMAState{}
				for index, sample := range samples {
					So(out[index], ShouldEqual, expect.Observe(sample))
				}
			})
		})
	})
}

func TestEMAState_Reset(testingTB *testing.T) {
	Convey("Given EMA state", testingTB, func() {
		state := EMAState{}
		_ = state.Observe(4)

		Convey("When reset", func() {
			state.Reset()

			Convey("It should clear readiness", func() {
				So(state.Ready, ShouldBeFalse)
			})
		})
	})
}

func BenchmarkEMAState_Observe(testingTB *testing.B) {
	state := EMAState{}
	sample := 1.0

	for testingTB.Loop() {
		sample = state.Observe(sample + 0.01)
	}
}

func BenchmarkEMAState_ObserveSamples(testingTB *testing.B) {
	state := EMAState{}
	samples := make([]float64, 1024)
	out := make([]float64, len(samples))

	for index := range samples {
		samples[index] = float64(index)
	}

	for testingTB.Loop() {
		state.Reset()
		state.ObserveSamples(samples, out)
	}
}

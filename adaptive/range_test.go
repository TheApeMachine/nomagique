package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestRangeState_Observe(testingTB *testing.T) {
	Convey("Given a fresh range state", testingTB, func() {
		state := RangeState{}

		Convey("When bootstrapping", func() {
			value := state.Observe(10)

			Convey("It should return zero", func() {
				So(value, ShouldEqual, 0)
			})
		})
	})

	Convey("Given range history", testingTB, func() {
		state := RangeState{}
		_ = state.Observe(10)

		Convey("When the sample extends the maximum", func() {
			value := state.Observe(25)

			Convey("It should return the new span", func() {
				So(value, ShouldEqual, 15)
			})
		})
	})
}

func TestRangeState_ObserveSamples(testingTB *testing.T) {
	Convey("Given samples", testingTB, func() {
		state := RangeState{}
		samples := []float64{10, 25}
		out := make([]float64, len(samples))

		Convey("When observing in batch", func() {
			state.ObserveSamples(samples, out)

			Convey("It should match sequential observation", func() {
				expect := RangeState{}
				for index, sample := range samples {
					So(out[index], ShouldEqual, expect.Observe(sample))
				}
			})
		})
	})
}

func TestRangeState_Reset(testingTB *testing.T) {
	Convey("Given range state", testingTB, func() {
		state := RangeState{}
		_ = state.Observe(3)

		Convey("When reset", func() {
			state.Reset()

			Convey("It should clear readiness", func() {
				So(state.Ready, ShouldBeFalse)
			})
		})
	})
}

func TestObserveRange(testingTB *testing.T) {
	Convey("Given ObserveRange", testingTB, func() {
		byFunction := RangeState{}
		byMethod := RangeState{}

		Convey("It should match method observation", func() {
			So(ObserveRange(&byFunction, 3), ShouldEqual, byMethod.Observe(3))
		})
	})
}

func BenchmarkRangeState_Observe(testingTB *testing.B) {
	state := RangeState{}
	_ = state.Observe(1)

	for testingTB.Loop() {
		_ = state.Observe(1.01)
	}
}

func BenchmarkRangeState_ObserveSamples(testingTB *testing.B) {
	state := RangeState{}
	samples := make([]float64, 1024)
	out := make([]float64, len(samples))

	for testingTB.Loop() {
		state.Reset()
		state.ObserveSamples(samples, out)
	}
}

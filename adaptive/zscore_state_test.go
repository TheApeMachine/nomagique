package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestZScoreState_Observe(testingTB *testing.T) {
	Convey("Given a fresh z-score state", testingTB, func() {
		state := ZScoreState{}

		Convey("When bootstrapping", func() {
			value := state.Observe(10)

			Convey("It should return zero", func() {
				So(value, ShouldEqual, 0)
			})
		})
	})

	Convey("Given z-score history", testingTB, func() {
		state := ZScoreState{}
		_ = state.Observe(0)

		Convey("When observing a step", func() {
			value := state.Observe(10)

			Convey("It should return a positive surprise", func() {
				So(value, ShouldBeGreaterThan, 0)
			})
		})
	})

	Convey("Given an anchored z-score", testingTB, func() {
		state := ZScoreState{}
		_ = ObserveZScore(&state, 0, 0, false)

		Convey("When anchored to a fixed level", func() {
			value := ObserveZScore(&state, 10, 0, true)

			Convey("It should score deviation from the anchor", func() {
				So(value, ShouldBeGreaterThan, 0)
			})
		})
	})
}

func TestZScoreState_ObserveSamples(testingTB *testing.T) {
	Convey("Given samples", testingTB, func() {
		state := ZScoreState{}
		samples := []float64{0, 10}
		out := make([]float64, len(samples))

		Convey("When observing in batch", func() {
			state.ObserveSamples(samples, out)

			Convey("It should match sequential observation", func() {
				expect := ZScoreState{}
				for index, sample := range samples {
					So(out[index], ShouldEqual, expect.Observe(sample))
				}
			})
		})
	})
}

func TestZScoreState_Reset(testingTB *testing.T) {
	Convey("Given z-score state", testingTB, func() {
		state := ZScoreState{}
		_ = state.Observe(3)

		Convey("When reset", func() {
			state.Reset()

			Convey("It should clear readiness", func() {
				So(state.Ready, ShouldBeFalse)
			})
		})
	})
}

func BenchmarkZScoreState_Observe(testingTB *testing.B) {
	state := ZScoreState{}
	_ = state.Observe(1)

	for testingTB.Loop() {
		_ = state.Observe(1.01)
	}
}

func BenchmarkZScoreState_ObserveSamples(testingTB *testing.B) {
	state := ZScoreState{}
	samples := make([]float64, 1024)
	out := make([]float64, len(samples))

	for testingTB.Loop() {
		state.Reset()
		state.ObserveSamples(samples, out)
	}
}

package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFracDiffState_Observe(testingTB *testing.T) {
	Convey("Given a fresh fractional differencing state", testingTB, func() {
		state := FracDiffState{}

		Convey("When bootstrapping", func() {
			value := state.Observe(10)

			Convey("It should return the first sample", func() {
				So(value, ShouldEqual, 10)
			})
		})
	})

	Convey("Given fractional differencing history", testingTB, func() {
		state := FracDiffState{}
		_ = state.Observe(10)
		value := state.Observe(12)

		Convey("It should emit a filtered value", func() {
			So(value, ShouldNotEqual, 0)
		})
	})

	Convey("Given a collapsed range", testingTB, func() {
		state := FracDiffState{}
		_ = state.Observe(8)
		value := state.Observe(8)

		Convey("It should keep filtering on a flat series", func() {
			So(value, ShouldEqual, 8)
		})
	})
}

func TestFracDiffState_ObserveSamples(testingTB *testing.T) {
	Convey("Given samples", testingTB, func() {
		state := FracDiffState{}
		samples := []float64{10, 12, 11, 15}
		out := make([]float64, len(samples))

		Convey("When observing in batch", func() {
			state.ObserveSamples(samples, out)

			Convey("It should match sequential observation", func() {
				expect := FracDiffState{}
				for index, sample := range samples {
					So(out[index], ShouldEqual, expect.Observe(sample))
				}
			})
		})
	})
}

func TestFracDiffState_Reset(testingTB *testing.T) {
	Convey("Given fractional differencing state", testingTB, func() {
		state := FracDiffState{}
		_ = state.Observe(3)

		Convey("When reset", func() {
			state.Reset()

			Convey("It should clear readiness", func() {
				So(state.Ready, ShouldBeFalse)
			})
		})
	})
}

func TestFracDiffState_ensureHistoryCapacity(testingTB *testing.T) {
	Convey("Given a growing history buffer", testingTB, func() {
		state := FracDiffState{
			Ready:   true,
			Head:    0,
			Count:   1,
			History: []float64{10},
		}

		Convey("When capacity increases", func() {
			state.ensureHistoryCapacity(4)

			Convey("It should preserve the newest sample", func() {
				So(len(state.History), ShouldEqual, 4)
				So(state.History[0], ShouldEqual, 10)
			})
		})
	})
}

func TestFracDiffState_pushHistory(testingTB *testing.T) {
	Convey("Given empty history storage", testingTB, func() {
		state := FracDiffState{}

		Convey("When pushing a sample", func() {
			state.pushHistory(1)

			Convey("It should remain a no-op", func() {
				So(state.History, ShouldBeNil)
			})
		})
	})
}

func BenchmarkFracDiffState_Observe(testingTB *testing.B) {
	state := FracDiffState{}
	_ = state.Observe(10)

	for testingTB.Loop() {
		_ = state.Observe(10.01)
	}
}

func BenchmarkFracDiffState_ObserveSamples(testingTB *testing.B) {
	state := FracDiffState{}
	samples := make([]float64, 1024)
	out := make([]float64, len(samples))

	for index := range samples {
		samples[index] = float64(index % 17)
	}

	for testingTB.Loop() {
		state.Reset()
		state.ObserveSamples(samples, out)
	}
}

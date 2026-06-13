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

func TestObserveFracDiff(testingTB *testing.T) {
	Convey("Given ObserveFracDiff", testingTB, func() {
		byFunction := FracDiffState{}
		byMethod := FracDiffState{}

		Convey("It should match method observation", func() {
			So(ObserveFracDiff(&byFunction, 3), ShouldEqual, byMethod.Observe(3))
		})
	})
}

func TestObserveFracDiffReady(testingTB *testing.T) {
	Convey("Given ready fractional differencing", testingTB, func() {
		state := FracDiffState{
			Ready:   true,
			Min:     10,
			Max:     20,
			Prev:    10,
			Order:   0.5,
			Width:   1,
			Head:    0,
			Count:   1,
			History: []float64{10},
			Weights: []float64{1},
		}

		Convey("When range expands", func() {
			value := observeFracDiffReady(&state, 15)

			Convey("It should filter with rebuilt weights", func() {
				So(value, ShouldNotEqual, 0)
			})
		})

	})

	Convey("Given ready state with a high minimum", testingTB, func() {
		state := FracDiffState{
			Ready:   true,
			Min:     10,
			Max:     20,
			Prev:    15,
			Order:   0.5,
			Width:   1,
			Head:    0,
			Count:   1,
			History: []float64{15},
			Weights: []float64{1},
		}

		Convey("When range contracts below the minimum", func() {
			value := observeFracDiffReady(&state, 5)

			Convey("It should still filter with updated weights", func() {
				So(value, ShouldNotEqual, 0)
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

func TestBuildFracDiffWeights_maxLag(testingTB *testing.T) {
	Convey("Given zero span with reference threshold", testingTB, func() {
		weights, width := buildFracDiffWeights(0.5, 0, 10, nil)

		Convey("It should exhaust the range-derived max lag", func() {
			So(width, ShouldEqual, 2)
			So(len(weights), ShouldEqual, 2)
		})
	})
}

func TestFracDiffOutput(testingTB *testing.T) {
	Convey("Given a partial history window", testingTB, func() {
		state := FracDiffState{
			Width:   3,
			Count:   1,
			Head:    0,
			History: []float64{5},
			Weights: []float64{1, -0.5, 0.1},
		}

		Convey("It should only convolve available lags", func() {
			So(fracDiffOutput(&state), ShouldEqual, 5)
		})
	})

	Convey("Given weights and history", testingTB, func() {
		state := FracDiffState{
			Ready:   true,
			Width:   2,
			Head:    1,
			Count:   2,
			History: []float64{10, 12},
			Weights: []float64{1, -0.4},
		}

		Convey("It should apply newest-first weights", func() {
			So(fracDiffOutput(&state), ShouldAlmostEqual, 8, 1e-12)
		})
	})

	Convey("Given a wrapped ring index", testingTB, func() {
		state := FracDiffState{
			Width:   2,
			Count:   2,
			Head:    0,
			History: []float64{10, 12},
			Weights: []float64{1, -0.4},
		}

		Convey("It should read older lags across the buffer end", func() {
			So(fracDiffOutput(&state), ShouldAlmostEqual, 5.2, 1e-12)
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

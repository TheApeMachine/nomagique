package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

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


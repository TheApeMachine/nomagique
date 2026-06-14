package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

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


package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFluidflowEvaluateLaminar(testingTB *testing.T) {
	Convey("Given a balanced laminar field", testingTB, func() {
		stage := NewFluidflow()
		outcome := stage.evaluate([]float64{
			0.5, 0.01, 0.8, 1, 1,
			2, 4, 0, 0.05, 0,
			100, 2, 0.01, 1000,
		})

		Convey("It should classify laminar flow", func() {
			So(outcome.Eligible, ShouldBeTrue)
			So(outcome.Category, ShouldEqual, 1)
			So(outcome.LaminarScore, ShouldBeGreaterThan, 0)
		})
	})
}

func TestFluidflowEvaluateTurbulent(testingTB *testing.T) {
	Convey("Given Reynolds above the turbulent floor", testingTB, func() {
		stage := NewFluidflow()
		outcome := stage.evaluate([]float64{
			8, 0.2, 0.5, 1, 1,
			2, 4, 1, 0.1, 0,
			100, 2, 0.01, 1000,
		})

		Convey("It should classify turbulent flow", func() {
			So(outcome.Eligible, ShouldBeTrue)
			So(outcome.Category, ShouldEqual, 2)
			So(outcome.TurbulentScore, ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkFluidflowEvaluate(testingTB *testing.B) {
	stage := NewFluidflow()
	batch := []float64{
		2, 0.1, 0.6, 1, 1,
		3, 5, 1, 0.08, 0,
		100, 2, 0.01, 1000,
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = stage.evaluate(batch)
	}
}

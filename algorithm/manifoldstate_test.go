package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestManifoldstateEvaluateHerd(testingTB *testing.T) {
	Convey("Given high coherence and guidance speed", testingTB, func() {
		stage := NewManifoldstate()
		outcome := stage.evaluate([]float64{
			0.5, 8, 1, 0.5, 50000,
		})

		Convey("It should classify systemic herd", func() {
			So(outcome.Eligible, ShouldBeTrue)
			So(outcome.Category, ShouldEqual, 1)
			So(outcome.HerdScore, ShouldBeGreaterThan, 0)
		})
	})
}

func TestManifoldstateEvaluateShock(testingTB *testing.T) {
	Convey("Given dominant pressure gradient", testingTB, func() {
		stage := NewManifoldstate()
		outcome := stage.evaluate([]float64{
			12, 0.2, 0.5, 0.5, 50000,
		})

		Convey("It should classify liquidity shock", func() {
			So(outcome.Eligible, ShouldBeTrue)
			So(outcome.Category, ShouldEqual, 2)
			So(outcome.ShockScore, ShouldBeGreaterThan, 0)
		})
	})
}

func TestManifoldstateEvaluateDrift(testingTB *testing.T) {
	Convey("Given laminar guidance with low viscosity", testingTB, func() {
		stage := NewManifoldstate()
		outcome := stage.evaluate([]float64{
			0.1, 0.2, 4, 0.1, 50000,
		})

		Convey("It should classify synchronized drift", func() {
			So(outcome.Eligible, ShouldBeTrue)
			So(outcome.Category, ShouldEqual, 3)
			So(outcome.DriftScore, ShouldBeGreaterThan, 0)
		})
	})
}

func TestManifoldstateEvaluateNoise(testingTB *testing.T) {
	Convey("Given low coherence and high viscosity", testingTB, func() {
		stage := NewManifoldstate()
		outcome := stage.evaluate([]float64{
			0.1, 0.1, 0.5, 2, 50000,
		})

		Convey("It should classify stochastic noise", func() {
			So(outcome.Eligible, ShouldBeTrue)
			So(outcome.Category, ShouldEqual, 4)
			So(outcome.NoiseScore, ShouldBeGreaterThan, 0)
		})
	})
}

func TestManifoldstateEvaluateIneligible(testingTB *testing.T) {
	Convey("Given non-positive observables", testingTB, func() {
		stage := NewManifoldstate()
		outcome := stage.evaluate([]float64{
			0.5, 0, 1, 0.5, 50000,
		})

		Convey("It should reject the reading", func() {
			So(outcome.Eligible, ShouldBeFalse)
		})
	})
}

func BenchmarkManifoldstateEvaluate(testingTB *testing.B) {
	stage := NewManifoldstate()
	batch := []float64{0.5, 8, 1, 0.5, 50000}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = stage.evaluate(batch)
	}
}

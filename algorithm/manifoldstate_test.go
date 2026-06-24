package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/tests"
)

func TestManifoldstateEvaluateHerd(testingTB *testing.T) {
	Convey("Given high coherence and guidance speed", testingTB, func() {
		stage := equation.NewManifoldstate(equation.ManifoldConfig())
		err := tests.WriteSamples(stage, 0.5, 8, 1, 0.5, 50000)

		So(err, ShouldBeNil)

		outbound, err := readOutbound(stage)

		So(err, ShouldBeNil)

		Convey("It should classify systemic herd", func() {
			So(datura.Peek[float64](outbound, "output", "value"), ShouldBeGreaterThan, 0)
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 1)
			So(datura.Peek[float64](outbound, "output", "herdScore"), ShouldBeGreaterThan, 0)
		})
	})
}

func TestManifoldstateEvaluateShock(testingTB *testing.T) {
	Convey("Given dominant pressure gradient", testingTB, func() {
		stage := equation.NewManifoldstate(equation.ManifoldConfig())
		err := tests.WriteSamples(stage, 12, 0.2, 0.5, 0.5, 50000)

		So(err, ShouldBeNil)

		outbound, err := readOutbound(stage)

		So(err, ShouldBeNil)

		Convey("It should classify liquidity shock", func() {
			So(datura.Peek[float64](outbound, "output", "value"), ShouldBeGreaterThan, 0)
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 2)
			So(datura.Peek[float64](outbound, "output", "shockScore"), ShouldBeGreaterThan, 0)
		})
	})
}

func TestManifoldstateEvaluateDrift(testingTB *testing.T) {
	Convey("Given laminar guidance with low viscosity", testingTB, func() {
		stage := equation.NewManifoldstate(equation.ManifoldConfig())
		err := tests.WriteSamples(stage, 0.1, 0.2, 4, 0.1, 50000)

		So(err, ShouldBeNil)

		outbound, err := readOutbound(stage)

		So(err, ShouldBeNil)

		Convey("It should classify synchronized drift", func() {
			So(datura.Peek[float64](outbound, "output", "value"), ShouldBeGreaterThan, 0)
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 3)
			So(datura.Peek[float64](outbound, "output", "driftScore"), ShouldBeGreaterThan, 0)
		})
	})
}

func TestManifoldstateEvaluateNoise(testingTB *testing.T) {
	Convey("Given low coherence and high viscosity", testingTB, func() {
		stage := equation.NewManifoldstate(equation.ManifoldConfig())
		err := tests.WriteSamples(stage, 0.1, 0.1, 0.5, 2, 50000)

		So(err, ShouldBeNil)

		outbound, err := readOutbound(stage)

		So(err, ShouldBeNil)

		Convey("It should classify stochastic noise", func() {
			So(datura.Peek[float64](outbound, "output", "value"), ShouldBeGreaterThan, 0)
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 4)
			So(datura.Peek[float64](outbound, "output", "noiseScore"), ShouldBeGreaterThan, 0)
		})
	})
}

func TestManifoldstateEvaluateIneligible(testingTB *testing.T) {
	Convey("Given non-positive observables", testingTB, func() {
		stage := equation.NewManifoldstate(equation.ManifoldConfig())
		err := tests.WriteSamples(stage, 0.5, 0, 1, 0.5, 50000)

		So(err, ShouldBeNil)

		_, err = readOutbound(stage)

		Convey("It should reject the reading", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func BenchmarkManifoldstateRead(testingTB *testing.B) {
	stage := equation.NewManifoldstate(equation.ManifoldConfig())
	batch := []float64{0.5, 8, 1, 0.5, 50000}
	frame := make([]byte, 4096)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = tests.WriteSamples(stage, batch...)
		_, _ = stage.Read(frame)
	}
}

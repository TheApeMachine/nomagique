package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/tests"
)

func TestFluidflowEvaluateLaminar(testingTB *testing.T) {
	Convey("Given a balanced laminar field", testingTB, func() {
		stage := equation.NewFluidflow()
		writeErr := tests.WriteSamples(stage,
			0.5, 0.01, 0.8, 1, 1,
			2, 4, 0, 0.05, 0, 0, 0,
			100, 2, 0.01, 1000,
		)

		So(writeErr, ShouldBeNil)

		outbound := readOutbound(stage)

		Convey("It should classify laminar flow", func() {
			So(datura.Peek[float64](outbound, "output", "value"), ShouldBeGreaterThan, 0)
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 1)
			So(datura.Peek[float64](outbound, "output", "laminarScore"), ShouldBeGreaterThan, 0)
		})
	})
}

func TestFluidflowEvaluateTurbulent(testingTB *testing.T) {
	Convey("Given Reynolds above the turbulent floor", testingTB, func() {
		stage := equation.NewFluidflow()
		writeErr := tests.WriteSamples(stage,
			8, 0.2, 0.5, 1, 1,
			2, 4, 1, 0.1, 0, 0.5, 0.8,
			100, 2, 0.01, 1000,
		)

		So(writeErr, ShouldBeNil)

		outbound := readOutbound(stage)

		Convey("It should classify turbulent flow", func() {
			So(datura.Peek[float64](outbound, "output", "value"), ShouldBeGreaterThan, 0)
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 2)
			So(datura.Peek[float64](outbound, "output", "turbulentScore"), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkFluidflowRead(testingTB *testing.B) {
	stage := equation.NewFluidflow()
	batch := []float64{
		2, 0.1, 0.6, 1, 1,
		3, 5, 1, 0.08, 0, 0.2, 0.3,
		100, 2, 0.01, 1000,
	}
	frame := make([]byte, 4096)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = tests.WriteSamples(stage, batch...)
		_, _ = stage.Read(frame)
	}
}

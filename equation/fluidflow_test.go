package equation_test

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/equation"
)

func TestFluidflow_Read(testingTB *testing.T) {
	Convey("Given a balanced laminar field", testingTB, func() {
		stage := equation.NewFluidflow(nil)
		writeErr := writeFeatureStage(stage, equation.FluidflowInputKeys,
			0.5, 0.01, 0.8, 1, 1,
			2, 4, 0, 0.05, 0, 0, 0, 0,
			100, 2, 0.01, 1000,
		)

		So(writeErr, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should classify laminar flow", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 1)
			So(datura.Peek[float64](outbound, "output", "value"), ShouldEqual, 0.5)
			So(datura.Peek[float64](outbound, "output", "laminarScore"), ShouldAlmostEqual, 0.64, 0.01)
		})
	})

	Convey("Given huge finite memory", testingTB, func() {
		stage := equation.NewFluidflow(nil)
		writeErr := writeFeatureStage(stage, equation.FluidflowInputKeys,
			0.5, 0.01, 0.8, 1, 1,
			2, 4, 0, 0.05, 0, 0, 0, math.MaxFloat64,
			100, 2, 0.01, 1000,
		)

		So(writeErr, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should keep viscousScore finite", func() {
			viscousScore := datura.Peek[float64](outbound, "output", "viscousScore")

			So(math.IsInf(viscousScore, 0), ShouldBeFalse)
			So(math.IsNaN(viscousScore), ShouldBeFalse)
		})
	})

	Convey("Given Reynolds above the turbulent floor", testingTB, func() {
		stage := equation.NewFluidflow(nil)
		writeErr := writeFeatureStage(stage, equation.FluidflowInputKeys,
			8, 0.2, 0.5, 1, 1,
			2, 4, 1, 0.1, 0, 0.5, 0.8, 0,
			100, 2, 0.01, 1000,
		)

		So(writeErr, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should classify turbulent flow", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 2)
			So(datura.Peek[float64](outbound, "output", "value"), ShouldEqual, 8)
			So(datura.Peek[float64](outbound, "output", "turbulentScore"), ShouldEqual, 6.4)
		})
	})
}

func BenchmarkFluidflowRead(b *testing.B) {
	stage := equation.NewFluidflow(nil)
	values := []float64{
		2, 0.1, 0.6, 1, 1,
		3, 5, 1, 0.08, 0, 0.2, 0.3, 0,
		100, 2, 0.01, 1000,
	}

	b.ReportAllocs()

	for b.Loop() {
		_ = writeFeatureStage(stage, equation.FluidflowInputKeys, values...)
		frame := make([]byte, 4096)
		_, _ = stage.Read(frame)
	}
}

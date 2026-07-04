package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/equation"
)

func TestFluidflowEvaluateLaminar(testingTB *testing.T) {
	Convey("Given a balanced laminar field", testingTB, func() {
		fluidflow := equation.NewFluidflow()
		output, err := fluidflow.Measure(fluidflowMeasureInput(
			0.5, 0.01, 0.8, 1, 1,
			2, 4, false, 0.05, 0, 0, 0, 0,
		))

		So(err, ShouldBeNil)

		Convey("It should classify laminar flow", func() {
			So(output.Value, ShouldBeGreaterThan, 0)
			So(int(output.Category), ShouldEqual, 1)
			So(output.LaminarScore, ShouldBeGreaterThan, 0)
		})
	})
}

func TestFluidflowEvaluateTurbulent(testingTB *testing.T) {
	Convey("Given Reynolds above the turbulent floor", testingTB, func() {
		fluidflow := equation.NewFluidflow()
		output, err := fluidflow.Measure(fluidflowMeasureInput(
			8, 0.2, 0.5, 1, 1,
			2, 4, true, 0.1, 0, 0.5, 0.8, 0,
		))

		So(err, ShouldBeNil)

		Convey("It should classify turbulent flow", func() {
			So(output.Value, ShouldBeGreaterThan, 0)
			So(int(output.Category), ShouldEqual, 2)
			So(output.TurbulentScore, ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkFluidflowMeasure(testingTB *testing.B) {
	fluidflow := equation.NewFluidflow()
	input := fluidflowMeasureInput(
		2, 0.1, 0.6, 1, 1,
		3, 5, true, 0.08, 0, 0.2, 0.3, 0,
	)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_, _ = fluidflow.Measure(input)
	}
}

func fluidflowMeasureInput(
	reynolds float64,
	divergence float64,
	viscosity float64,
	midAddRate float64,
	midExecuteRate float64,
	laminarCeiling float64,
	turbulentFloor float64,
	turbulentReady bool,
	divergenceEdge float64,
	icebergScore float64,
	vorticity float64,
	turbulence float64,
	memory float64,
) equation.FluidflowInput {
	return equation.FluidflowInput{
		Reynolds:       reynolds,
		Divergence:     divergence,
		Viscosity:      viscosity,
		MidAddRate:     midAddRate,
		MidExecuteRate: midExecuteRate,
		LaminarCeiling: laminarCeiling,
		TurbulentFloor: turbulentFloor,
		TurbulentReady: turbulentReady,
		DivergenceEdge: divergenceEdge,
		IcebergScore:   icebergScore,
		Vorticity:      vorticity,
		Turbulence:     turbulence,
		Memory:         memory,
		Price:          100,
		SpreadBPS:      2,
		ChangePct:      0.01,
		Volume:         1000,
	}
}

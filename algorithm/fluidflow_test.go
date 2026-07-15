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
			0.5, 0.01, 0.8, 2, 4, 0.05, 0.8, 0, 0, 0, 0,
		))

		So(err, ShouldBeNil)

		Convey("It should measure laminar evidence", func() {
			So(output.LaminarScore, ShouldEqual, 1)
			So(output.TurbulentScore, ShouldEqual, 0)
		})
	})
}

func TestFluidflowEvaluateTurbulent(testingTB *testing.T) {
	Convey("Given Reynolds above the turbulent floor", testingTB, func() {
		fluidflow := equation.NewFluidflow()
		output, err := fluidflow.Measure(fluidflowMeasureInput(
			8, 0.2, 0.5, 2, 4, 0.1, 0.5, 0.5, 0.1, 0.8, 0.2,
		))

		So(err, ShouldBeNil)

		Convey("It should measure turbulent evidence", func() {
			So(output.TurbulentScore, ShouldEqual, 4)
			So(output.InertialScore, ShouldEqual, 1)
		})
	})
}

func BenchmarkFluidflowMeasure(testingTB *testing.B) {
	fluidflow := equation.NewFluidflow()
	input := fluidflowMeasureInput(
		2, 0.1, 0.6, 3, 5, 0.08, 0.5, 0.2, 0.1, 0.3, 0.2,
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
	laminarCeiling float64,
	turbulentFloor float64,
	divergenceEdge float64,
	viscosityBaseline float64,
	vorticity float64,
	vorticityBaseline float64,
	turbulence float64,
	turbulenceBaseline float64,
) equation.FluidflowInput {
	return equation.FluidflowInput{
		Reynolds:           reynolds,
		Divergence:         divergence,
		Viscosity:          viscosity,
		LaminarCeiling:     laminarCeiling,
		TurbulentFloor:     turbulentFloor,
		DivergenceEdge:     divergenceEdge,
		ViscosityBaseline:  viscosityBaseline,
		Vorticity:          vorticity,
		VorticityBaseline:  vorticityBaseline,
		Turbulence:         turbulence,
		TurbulenceBaseline: turbulenceBaseline,
	}
}

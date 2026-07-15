package equation_test

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/equation"
)

func TestFluidflow_Measure(testingTB *testing.T) {
	Convey("Given a balanced laminar field", testingTB, func() {
		fluidflow := equation.NewFluidflow()
		output, err := fluidflow.Measure(fluidflowInput(
			0.5, 0.01, 0.8, 2, 4, 0.05, 0.8, 0, 0, 0, 0,
		))

		So(err, ShouldBeNil)

		Convey("It should measure laminar evidence without selecting a category", func() {
			So(output.LaminarScore, ShouldEqual, 1)
			So(output.TurbulentScore, ShouldEqual, 0)
			So(output.InertialScore, ShouldEqual, 0)
			So(output.ViscousScore, ShouldEqual, 0)
		})
	})

	Convey("Given huge finite viscosity and its empirical baseline", testingTB, func() {
		fluidflow := equation.NewFluidflow()
		output, err := fluidflow.Measure(fluidflowInput(
			0.5, 0.01, math.MaxFloat64, 2, 4, 0.05,
			math.MaxFloat64, 0, 0, 0, 0,
		))

		So(err, ShouldBeNil)

		Convey("It should keep viscousScore finite", func() {
			So(math.IsInf(output.ViscousScore, 0), ShouldBeFalse)
			So(math.IsNaN(output.ViscousScore), ShouldBeFalse)
		})
	})

	Convey("Given Reynolds above the turbulent floor", testingTB, func() {
		fluidflow := equation.NewFluidflow()
		output, err := fluidflow.Measure(fluidflowInput(
			8, 0.2, 0.5, 2, 4, 0.1, 0.5, 0.5, 0.1, 0.8, 0.2,
		))

		So(err, ShouldBeNil)

		Convey("It should measure turbulent evidence above empirical baselines", func() {
			So(output.TurbulentScore, ShouldEqual, 4)
			So(output.InertialScore, ShouldEqual, 1)
			So(output.LaminarScore, ShouldAlmostEqual, 0.2)
		})
	})

	Convey("Given zero field motion with positive viscosity", testingTB, func() {
		fluidflow := equation.NewFluidflow()
		output, err := fluidflow.Measure(equation.FluidflowInput{
			Viscosity:         1000,
			ViscosityBaseline: 1000,
		})

		So(err, ShouldBeNil)

		Convey("It should measure baseline laminar stability", func() {
			So(output.LaminarScore, ShouldEqual, 1)
		})
	})
}

func BenchmarkFluidflowMeasure(benchmark *testing.B) {
	fluidflow := equation.NewFluidflow()
	input := fluidflowInput(
		2, 0.1, 0.6, 3, 5, 0.08, 0.5, 0.2, 0.1, 0.3, 0.2,
	)

	benchmark.ReportAllocs()

	for benchmark.Loop() {
		_, _ = fluidflow.Measure(input)
	}
}

func fluidflowInput(
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

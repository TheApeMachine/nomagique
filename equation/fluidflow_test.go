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
			0.5, 0.01, 0.8, 1, 1,
			2, 4, false, 0.05, 0, 0, 0, 0,
		))

		So(err, ShouldBeNil)

		Convey("It should classify laminar flow", func() {
			So(int(output.Category), ShouldEqual, 1)
			So(output.Value, ShouldEqual, 0.5)
			So(output.LaminarScore, ShouldAlmostEqual, 0.64, 0.01)
		})
	})

	Convey("Given huge finite memory", testingTB, func() {
		fluidflow := equation.NewFluidflow()
		output, err := fluidflow.Measure(fluidflowInput(
			0.5, 0.01, 0.8, 1, 1,
			2, 4, false, 0.05, 0, 0, 0, math.MaxFloat64,
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
			8, 0.2, 0.5, 1, 1,
			2, 4, true, 0.1, 0, 0.5, 0.8, 0,
		))

		So(err, ShouldBeNil)

		Convey("It should classify turbulent flow", func() {
			So(int(output.Category), ShouldEqual, 2)
			So(output.Value, ShouldEqual, 8)
			So(output.TurbulentScore, ShouldEqual, 6.4)
		})
	})

	Convey("Given a valid flat price-change field", testingTB, func() {
		fluidflow := equation.NewFluidflow()
		output, err := fluidflow.Measure(fluidflowInput(
			0.5, 0.01, 0.8, 1, 1,
			2, 4, false, 0.05, 0, 0, 0, 0,
		))

		So(err, ShouldBeNil)

		Convey("It should not reject zero changePct", func() {
			So(int(output.Category), ShouldEqual, 1)
			So(output.ChangePct, ShouldEqual, 0)
		})
	})

	Convey("Given zero field motion with positive viscosity", testingTB, func() {
		fluidflow := equation.NewFluidflow()
		output, err := fluidflow.Measure(equation.FluidflowInput{
			Viscosity: 1000,
			Price:     100,
			SpreadBPS: 2,
			Volume:    1000,
		})

		So(err, ShouldBeNil)

		Convey("It should classify laminar stability", func() {
			So(int(output.Category), ShouldEqual, 1)
			So(output.LaminarScore, ShouldEqual, 1000)
		})
	})
}

func BenchmarkFluidflowMeasure(benchmark *testing.B) {
	fluidflow := equation.NewFluidflow()
	input := fluidflowInput(
		2, 0.1, 0.6, 1, 1,
		3, 5, true, 0.08, 0, 0.2, 0.3, 0,
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
		ChangePct:      0,
		Volume:         1000,
	}
}

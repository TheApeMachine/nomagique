package learning

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/tests"
)

func TestWeight(testingTB *testing.T) {
	Convey("Given Weight constructor", testingTB, func() {
		trustWeight := Weight[float64]()

		Convey("It should return a usable dynamic", func() {
			So(trustWeight, ShouldNotBeNil)
		})
	})
}

func TestTrustWeight_Observe(testingTB *testing.T) {
	Convey("Given empty Observe inputs", testingTB, func() {
		trustWeight := Weight[float64]()

		Convey("It should return zero output", func() {
			So(trustWeight.Observe(), ShouldEqual, core.Scalar[float64](0))
		})
	})

	Convey("Given a fresh trust weight", testingTB, func() {
		trustWeight := Weight[float64]()
		got := trustWeight.Observe(numberInputs(10, 10)...)

		Convey("It should return full trust", func() {
			So(float64(got), ShouldEqual, 1)
		})
	})

	Convey("Given diverging outcomes", testingTB, func() {
		trustWeight := Weight[float64]()
		_ = trustWeight.Observe(numberInputs(10, 10)...)
		got := trustWeight.Observe(numberInputs(20, 30)...)

		Convey("It should reduce trust", func() {
			So(float64(got), ShouldBeLessThan, 1)
		})
	})

	Convey("Given zero predicted", testingTB, func() {
		trustWeight := Weight[float64]()
		got := trustWeight.Observe(numberInputs(0, 10)...)

		Convey("It should leave output at zero", func() {
			So(float64(got), ShouldEqual, 0)
		})
	})

	Convey("Given a non-scalar first input", testingTB, func() {
		trustWeight := Weight[float64]()
		before := trustWeight.Observe(numberInputs(10, 10)...)
		stage := &tests.PipelineStage[float64]{Result: core.Scalar[float64](99)}

		Convey("It should leave output unchanged", func() {
			So(trustWeight.Observe(stage), ShouldEqual, before)
		})
	})
}

func TestTrustWeight_Reset(testingTB *testing.T) {
	Convey("Given trust weight with state", testingTB, func() {
		trustWeight := Weight[float64]()
		_ = trustWeight.Observe(numberInputs(10, 10)...)

		So(trustWeight.Reset(), ShouldBeNil)

		Convey("It should clear derived state", func() {
			So(trustWeight.state.Ready, ShouldBeFalse)
			So(float64(trustWeight.Observe()), ShouldEqual, 0)
		})
	})
}

func BenchmarkWeight_Observe(testingTB *testing.B) {
	trustWeight := Weight[float64]()
	_ = trustWeight.Observe(numberInputs(10, 10)...)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = trustWeight.Observe(numberInputs(10, 11)...)
	}
}

func BenchmarkWeight_ObserveSamples(testingTB *testing.B) {
	trustWeight := Weight[float64]()
	predicted := make([]float64, 1024)
	actual := make([]float64, len(predicted))
	out := make([]float64, len(predicted))

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		trustWeight.state.Reset()
		trustWeight.ObserveSamples(predicted, actual, out)
	}
}

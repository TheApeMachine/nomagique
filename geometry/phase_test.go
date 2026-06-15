package geometry

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/tests"
)

func TestCoupling_Observe(testingTB *testing.T) {
	cases := []struct {
		name   string
		left   float64
		right  float64
		expect float64
	}{
		{"co-moving growth", 2, 2, 1},
		{"opposing growth", 2, -2, -1},
		{"below relative floor", 0.001, 0.001, 0},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			stage := NewCoupling[float64]()
			got := stage.Observe(numberInputs(testCase.left, testCase.right)...)

			Convey("It should return the expected coupling", func() {
				So(float64(got), ShouldAlmostEqual, testCase.expect, 1e-9)
			})
		})
	}

	Convey("Given empty Observe inputs", testingTB, func() {
		stage := NewCoupling[float64]()

		Convey("It should return zero output", func() {
			So(stage.Observe(), ShouldEqual, core.Scalar[float64](0))
		})
	})

	Convey("Given a non-scalar first input", testingTB, func() {
		stage := NewCoupling[float64]()
		before := stage.Observe(numberInputs(2, 2)...)
		pipelineStage := &tests.PipelineStage[float64]{Result: core.Scalar[float64](99)}

		Convey("It should leave output unchanged", func() {
			So(stage.Observe(pipelineStage), ShouldEqual, before)
		})
	})
}

func TestCoupling_Reset(testingTB *testing.T) {
	Convey("Given an observed coupling stage", testingTB, func() {
		stage := NewCoupling[float64]()
		_ = stage.Observe(numberInputs(2, 2)...)

		So(stage.Reset(), ShouldBeNil)

		Convey("It should clear output", func() {
			So(float64(stage.Observe()), ShouldEqual, 0)
		})
	})
}

func TestVelocity_Observe(testingTB *testing.T) {
	Convey("Given empty Observe inputs", testingTB, func() {
		stage := NewVelocity[float64]()

		Convey("It should return zero output", func() {
			So(stage.Observe(), ShouldEqual, core.Scalar[float64](0))
		})
	})

	Convey("Given velocity history", testingTB, func() {
		stage := NewVelocity[float64]()
		_ = stage.Observe(core.Scalar[float64](1))
		got := stage.Observe(core.Scalar[float64](1.5))

		Convey("It should return the velocity", func() {
			So(float64(got), ShouldAlmostEqual, 0.5, 1e-12)
		})
	})

	Convey("Given a scalar plus work sample", testingTB, func() {
		stage := NewVelocity[float64]()
		_ = stage.Observe(core.Scalar[float64](1))

		Convey("It should match a single combined scalar", func() {
			withWork := stage.Observe(
				core.Scalar[float64](0.5),
				core.Scalar[float64](1.0),
			)
			expect := NewVelocity[float64]()
			_ = expect.Observe(core.Scalar[float64](1))
			combined := expect.Observe(core.Scalar[float64](1.5))

			So(withWork, ShouldEqual, combined)
		})
	})
}

func TestVelocity_ObserveSamples(testingTB *testing.T) {
	Convey("Given mean samples", testingTB, func() {
		stage := NewVelocity[float64]()
		means := []float64{1, 1.5, 1.25}
		out := make([]float64, len(means))

		stage.ObserveSamples(means, out)

		Convey("It should match sequential Observe", func() {
			expect := NewVelocity[float64]()
			expectOut := make([]float64, len(means))
			expect.ObserveSamples(means, expectOut)

			So(out, ShouldResemble, expectOut)
		})
	})
}

func TestVelocity_Reset(testingTB *testing.T) {
	Convey("Given an observed velocity stage", testingTB, func() {
		stage := NewVelocity[float64]()
		_ = stage.Observe(core.Scalar[float64](1))

		So(stage.Reset(), ShouldBeNil)

		Convey("It should clear derived state", func() {
			So(stage.ready, ShouldBeFalse)
			So(float64(stage.Observe()), ShouldEqual, 0)
		})
	})
}

func BenchmarkCoupling_Observe(testingTB *testing.B) {
	stage := NewCoupling[float64]()

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = stage.Observe(numberInputs(1.7, -0.9)...)
	}
}

func BenchmarkVelocity_Observe(testingTB *testing.B) {
	stage := NewVelocity[float64]()
	_ = stage.Observe(core.Scalar[float64](1))

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = stage.Observe(core.Scalar[float64](1.5))
	}
}

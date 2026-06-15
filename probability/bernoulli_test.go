package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/tests"
)

func TestBernoulli(testingTB *testing.T) {
	Convey("Given Bernoulli constructor", testingTB, func() {
		posterior := Bernoulli[float64]()

		Convey("It should return a usable dynamic", func() {
			So(posterior, ShouldNotBeNil)
		})
	})
}

func TestPosterior_Observe(testingTB *testing.T) {
	cases := []struct {
		name   string
		inputs []float64
		expect func(float64) bool
	}{
		{
			name:   "unit success",
			inputs: []float64{1},
			expect: func(value float64) bool { return value > 0.5 },
		},
		{
			name:   "unit failure",
			inputs: []float64{0},
			expect: func(value float64) bool { return value < 0.5 },
		},
		{
			name:   "partial outcome",
			inputs: []float64{0.75},
			expect: func(value float64) bool { return value > 0.5 && value < 1 },
		},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			posterior := Bernoulli[float64]()
			got := posterior.Observe(numberInputs(testCase.inputs...)...)

			Convey("It should return the expected posterior mean", func() {
				So(testCase.expect(float64(got)), ShouldBeTrue)
			})
		})
	}

	Convey("Given empty Observe inputs", testingTB, func() {
		posterior := Bernoulli[float64]()

		Convey("It should return zero output", func() {
			So(posterior.Observe(), ShouldEqual, core.Scalar[float64](0))
		})
	})

	Convey("Given a non-scalar first input", testingTB, func() {
		posterior := Bernoulli[float64]()
		before := posterior.Observe(core.Scalar[float64](1))
		stage := &tests.PipelineStage[float64]{Result: core.Scalar[float64](99)}

		Convey("It should leave output unchanged", func() {
			So(posterior.Observe(stage), ShouldEqual, before)
		})
	})

	Convey("Given a predicted and actual pair", testingTB, func() {
		posterior := Bernoulli[float64]()
		_ = posterior.Observe(numberInputs(10, 10)...)
		got := posterior.Observe(numberInputs(10, 15)...)

		Convey("It should raise hit probability", func() {
			So(float64(got), ShouldBeGreaterThan, 0.5)
		})
	})

	Convey("Given an invalid outcome", testingTB, func() {
		posterior := Bernoulli[float64]()
		got := posterior.Observe(core.Scalar[float64](2))

		Convey("It should leave output at zero", func() {
			So(float64(got), ShouldEqual, 0)
		})
	})
}

func TestPosterior_Reset(testingTB *testing.T) {
	Convey("Given an observed posterior", testingTB, func() {
		posterior := Bernoulli[float64]()
		_ = posterior.Observe(core.Scalar[float64](1))

		So(posterior.Reset(), ShouldBeNil)

		Convey("It should clear derived state", func() {
			So(posterior.state.Ready, ShouldBeFalse)
			So(float64(posterior.Observe()), ShouldEqual, 0)
		})
	})
}

func BenchmarkBernoulli_Observe(testingTB *testing.B) {
	posterior := Bernoulli[float64]()
	_ = posterior.Observe(core.Scalar[float64](1))

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = posterior.Observe(numberInputs(10, 11)...)
	}
}

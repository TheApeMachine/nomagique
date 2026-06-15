package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/tests"
)

func TestNewSum(testingTB *testing.T) {
	Convey("Given NewSum", testingTB, func() {
		sum := NewSum[float64]()

		Convey("It should return a usable stage", func() {
			So(sum, ShouldNotBeNil)
		})
	})
}

func TestSum_Observe(testingTB *testing.T) {
	cases := []struct {
		name    string
		samples []float64
		expect  float64
	}{
		{"positive batch", []float64{1.2, 0.8, 3.0}, 5.0},
		{"negative mix", []float64{-2, 5, -1}, 2},
		{"single value", []float64{7}, 7},
		{"empty input", nil, 0},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			sum := NewSum[float64]()
			got := sum.Observe(numberInputs(testCase.samples...)...)

			Convey("It should return the expected sum", func() {
				So(float64(got), ShouldEqual, testCase.expect)
			})
		})
	}

	Convey("Given a non-scalar input", testingTB, func() {
		sum := NewSum[float64]()
		_ = sum.Observe(core.Scalar[float64](10))
		stage := &tests.PipelineStage[float64]{Result: core.Scalar[float64](99)}

		Convey("It should ignore non-scalar inputs", func() {
			So(float64(sum.Observe(stage)), ShouldEqual, 10)
		})
	})
}

func TestSum_Reset(testingTB *testing.T) {
	Convey("Given an observed sum", testingTB, func() {
		sum := NewSum[float64]()
		_ = sum.Observe(numberInputs(1, 2, 3)...)

		So(sum.Reset(), ShouldBeNil)

		Convey("It should clear output", func() {
			So(float64(sum.Observe()), ShouldEqual, 0)
		})
	})
}

func BenchmarkSum_Observe(b *testing.B) {
	sum := NewSum[float64]()
	inputs := numberInputs(1.2, 0.8, 3.0, 4.0, 5.0)

	b.ReportAllocs()

	for b.Loop() {
		_ = sum.Observe(inputs...)
	}
}

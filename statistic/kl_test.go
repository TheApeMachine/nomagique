package statistic

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestKLDivergence_Observe(testingTB *testing.T) {
	Convey("Given aligned observed and expected halves", testingTB, func() {
		kl := NewKLDivergence[float64](nil, 0, 0)
		inputs := append(
			numberInputs(0.25, 0.25, 0.25, 0.25),
			numberInputs(0.25, 0.25, 0.25, 0.25)...,
		)
		got := kl.Observe(inputs...)

		Convey("It should return zero divergence for identical distributions", func() {
			So(float64(got), ShouldEqual, 0)
		})
	})

	errorCases := []struct {
		name   string
		inputs []core.Number[float64]
	}{
		{"single input", numberInputs(1)},
		{"odd input count", numberInputs(1, 2, 3)},
		{"non-finite observed", []core.Number[float64]{
			core.Scalar[float64](1),
			core.Scalar[float64](math.NaN()),
			core.Scalar[float64](1),
			core.Scalar[float64](1),
		}},
	}

	for _, testCase := range errorCases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			kl := NewKLDivergence[float64](nil, 0, 0)
			got := kl.Observe(testCase.inputs...)

			Convey("It should leave output unchanged", func() {
				So(float64(got), ShouldEqual, 0)
			})
		})
	}

	Convey("Given mismatched halves with floor", testingTB, func() {
		kl := NewKLDivergence[float64](nil, 1, 1e-6)
		inputs := append(numberInputs(1, 0), numberInputs(0, 1)...)
		got := kl.Observe(inputs...)

		Convey("It should still return a finite divergence", func() {
			So(float64(got), ShouldBeGreaterThan, 0)
		})
	})
}

func TestKLDivergence_Reset(testingTB *testing.T) {
	Convey("Given an observed KL stage", testingTB, func() {
		kl := NewKLDivergence[float64]([]float64{1}, 0, 0)
		_ = kl.Observe(append(numberInputs(1, 1), numberInputs(1, 1)...)...)

		So(kl.Reset(), ShouldBeNil)

		Convey("It should clear weights", func() {
			So(kl.weights, ShouldBeNil)
		})
	})
}

func BenchmarkKLDivergence_Observe(b *testing.B) {
	kl := NewKLDivergence[float64](nil, 0, 0)
	inputs := append(
		numberInputs(0.25, 0.25, 0.25, 0.25),
		numberInputs(0.2, 0.2, 0.3, 0.3)...,
	)

	b.ReportAllocs()

	for b.Loop() {
		_ = kl.Observe(inputs...)
	}
}

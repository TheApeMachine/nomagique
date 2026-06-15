package statistic

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestKLDivergence_Observe(testingTB *testing.T) {
	Convey("Given aligned observed and expected halves", testingTB, func() {
		kl := NewKLDivergence(nil, 0, 0)
		got := observeInputs(
			kl,
			0.25, 0.25, 0.25, 0.25,
			0.25, 0.25, 0.25, 0.25,
		)

		Convey("It should return zero divergence for identical distributions", func() {
			So(float64(got), ShouldEqual, 0)
		})
	})

	errorCases := []struct {
		name   string
		series []float64
	}{
		{"single input", []float64{1}},
		{"odd input count", []float64{1, 2, 3}},
		{"non-finite observed", []float64{1, math.NaN(), 1, 1}},
	}

	for _, testCase := range errorCases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			kl := NewKLDivergence(nil, 0, 0)
			got := observeInputs(kl, testCase.series...)

			Convey("It should leave output unchanged", func() {
				So(float64(got), ShouldEqual, 0)
			})
		})
	}

	Convey("Given mismatched halves with floor", testingTB, func() {
		kl := NewKLDivergence(nil, 1, 1e-6)
		got := observeInputs(kl, 1, 0, 0, 1)

		Convey("It should still return a finite divergence", func() {
			So(float64(got), ShouldBeGreaterThan, 0)
		})
	})
}

func TestKLDivergence_Reset(testingTB *testing.T) {
	Convey("Given an observed KL stage", testingTB, func() {
		kl := NewKLDivergence([]float64{1}, 0, 0)
		_ = observeInputs(kl, 1, 1, 1, 1)

		So(kl.Reset(), ShouldBeNil)

		Convey("It should clear weights", func() {
			So(kl.weights, ShouldBeNil)
		})
	})
}

func BenchmarkKLDivergence_Observe(b *testing.B) {
	kl := NewKLDivergence(nil, 0, 0)

	b.ReportAllocs()

	for b.Loop() {
		_ = observeInputs(
			kl,
			0.25, 0.25, 0.25, 0.25,
			0.2, 0.2, 0.3, 0.3,
		)
	}
}

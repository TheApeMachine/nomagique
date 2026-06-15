package statistic

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestMedian_Observe(testingTB *testing.T) {
	cases := []struct {
		name    string
		samples []float64
		expect  float64
	}{
		{"odd length", []float64{3, 1, 2}, 2},
		{"even length", []float64{1, 2, 3, 4}, 2.5},
		{"empty input", nil, 0},
		{"single value", []float64{7}, 7},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			median := NewMedian[float64](nil)
			got := median.Observe(numberInputs(testCase.samples...)...)

			Convey("It should return the expected median", func() {
				So(float64(got), ShouldEqual, testCase.expect)
			})
		})
	}

	Convey("Given weighted samples", testingTB, func() {
		median := NewMedian[float64]([]float64{1, 1, 1, 3})
		got := median.Observe(numberInputs(1, 1, 1, 3)...)

		Convey("It should apply weights", func() {
			So(float64(got), ShouldEqual, 1)
		})
	})

	Convey("Given mismatched weights", testingTB, func() {
		median := NewMedian[float64]([]float64{1, 1})
		got := median.Observe(numberInputs(1, 2, 3)...)

		Convey("It should leave output unchanged", func() {
			So(float64(got), ShouldEqual, 0)
		})
	})

	Convey("Given non-finite samples", testingTB, func() {
		median := NewMedian[float64](nil)
		got := median.Observe(
			core.Scalar[float64](1),
			core.Scalar[float64](math.NaN()),
			core.Scalar[float64](3),
		)

		Convey("It should return NaN", func() {
			So(math.IsNaN(float64(got)), ShouldBeTrue)
		})
	})
}

func TestMedianOf(testingTB *testing.T) {
	Convey("Given unsorted values", testingTB, func() {
		Convey("It should match Observe on the same batch", func() {
			So(MedianOf([]float64{3, 1, 2}), ShouldEqual, 2)
		})
	})
}

func TestMedian_Reset(testingTB *testing.T) {
	Convey("Given an observed median", testingTB, func() {
		median := NewMedian[float64]([]float64{1})
		_ = median.Observe(numberInputs(1)...)

		So(median.Reset(), ShouldBeNil)

		Convey("It should clear weights", func() {
			So(median.weights, ShouldBeNil)
		})
	})
}

func BenchmarkMedian_Observe(b *testing.B) {
	median := NewMedian[float64](nil)
	inputs := numberInputs(3, 1, 2, 4, 5)

	b.ReportAllocs()

	for b.Loop() {
		_ = median.Observe(inputs...)
	}
}

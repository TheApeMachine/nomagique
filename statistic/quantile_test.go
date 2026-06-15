package statistic

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"gonum.org/v1/gonum/stat"
)

func TestQuantile_Observe(testingTB *testing.T) {
	cases := []struct {
		name       string
		percentile float64
		kind       stat.CumulantKind
		samples    []float64
		expect     float64
	}{
		{"median lininterp", 0.5, stat.LinInterp, []float64{1, 2, 3, 4}, 2},
		{"lower quartile", 0.25, stat.LinInterp, []float64{1, 2, 3, 4}, 1},
		{"empty input", 0.5, stat.LinInterp, nil, 0},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			quantile := NewQuantile(
				testCase.percentile, testCase.kind, nil,
			)
			got := observeInputs(quantile, testCase.samples...)

			Convey("It should return the expected quantile", func() {
				So(float64(got), ShouldEqual, testCase.expect)
			})
		})
	}

	Convey("Given empirical weighted quantile", testingTB, func() {
		quantile := NewQuantile(0.5, stat.Empirical, []float64{1, 1, 1, 3})
		got := observeInputs(quantile, 1, 1, 1, 3)

		Convey("It should apply weights", func() {
			So(float64(got), ShouldEqual, 1)
		})
	})

	Convey("Given non-finite samples", testingTB, func() {
		quantile := NewQuantile(0.5, stat.LinInterp, nil)
		got := observeInputs(quantile, 1, math.NaN(), 3)

		Convey("It should return NaN", func() {
			So(math.IsNaN(float64(got)), ShouldBeTrue)
		})
	})
}

func TestQuantile_Reset(testingTB *testing.T) {
	Convey("Given an observed quantile", testingTB, func() {
		quantile := NewQuantile(0.5, stat.LinInterp, []float64{1, 2})
		_ = observeInputs(quantile, 1, 2)

		So(quantile.Reset(), ShouldBeNil)

		Convey("It should clear weights", func() {
			So(quantile.weights, ShouldBeNil)
		})
	})
}

func BenchmarkQuantile_Observe(b *testing.B) {
	quantile := NewQuantile(0.5, stat.LinInterp, nil)

	b.ReportAllocs()

	for b.Loop() {
		_ = observeInputs(quantile)
	}
}

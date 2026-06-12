package statistic

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/core"
	"gonum.org/v1/gonum/stat"
)

func TestQuantile_Observe(testingTB *testing.T) {
	Convey("Given an even-length stream with LinInterp at p=0.5", testingTB, func() {
		quantile := NewQuantile(0.5, stat.LinInterp, nil).Observe(
			nomagique.Numbers(1, 2, 3, 4)...,
		)

		Convey("It should use gonum linear interpolation", func() {
			So(float64(quantile), ShouldEqual, 2)
		})
	})

	Convey("Given a sorted slice with LinInterp at p=0.25", testingTB, func() {
		quantile := NewQuantile(0.25, stat.LinInterp, nil).ObserveSorted(
			[]float64{1, 2, 3, 4},
		)

		Convey("It should return the lower interpolated quartile", func() {
			So(float64(quantile), ShouldEqual, 1)
		})
	})

	Convey("Given an empty stream", testingTB, func() {
		quantile := NewQuantile(0.5, stat.LinInterp, nil).Observe()

		Convey("It should return zero", func() {
			So(float64(quantile), ShouldEqual, 0)
		})
	})

	Convey("Given a weighted stream with Empirical at p=0.5", testingTB, func() {
		quantile := NewQuantile(0.5, stat.Empirical, nomagique.Numbers(1, 1, 1, 3)).Observe(
			nomagique.Numbers(1, 2, 3, 100)...,
		)

		Convey("It should follow gonum empirical quantile", func() {
			So(float64(quantile), ShouldEqual, 3)
		})
	})

	Convey("Given mismatched weights", testingTB, func() {
		quantile := NewQuantile(0.5, stat.LinInterp, nomagique.Numbers(1, 1)).Observe(
			nomagique.Numbers(1, 2, 3)...,
		)

		Convey("It should return zero", func() {
			So(float64(quantile), ShouldEqual, 0)
		})
	})

	Convey("Given a non-finite sample", testingTB, func() {
		quantile := NewQuantile(0.5, stat.LinInterp, nil).Observe(
			core.Float64(1), core.Float64(math.NaN()), core.Float64(3),
		)

		Convey("It should return NaN", func() {
			So(math.IsNaN(float64(quantile)), ShouldBeTrue)
		})
	})
}

func TestQuartiles_Observe(testingTB *testing.T) {
	Convey("Given a four-value stream", testingTB, func() {
		lower, upper := NewQuartiles(stat.LinInterp, nil).Observe(
			nomagique.Numbers(1, 2, 3, 4)...,
		)

		Convey("It should return gonum LinInterp quartiles", func() {
			So(float64(lower), ShouldEqual, 1)
			So(float64(upper), ShouldEqual, 3)
		})
	})
}

func BenchmarkQuantile_Observe(testingTB *testing.B) {
	inputs := nomagique.Numbers(1, 2, 3, 4, 5, 6, 7, 8)
	quantile := NewQuantile(0.5, stat.LinInterp, nil)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = quantile.Observe(inputs...)
	}
}

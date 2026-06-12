package statistic

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/core"
)

func TestMedian_Observe(testingTB *testing.T) {
	Convey("Given an odd-length unweighted stream", testingTB, func() {
		median := NewMedian(nil).Observe(nomagique.Numbers(3, 1, 2)...)

		Convey("It should return the middle order statistic", func() {
			So(float64(median), ShouldEqual, 2)
		})
	})

	Convey("Given an even-length unweighted stream", testingTB, func() {
		median := NewMedian(nil).Observe(nomagique.Numbers(1, 2, 3, 4)...)

		Convey("It should average the two central values", func() {
			So(float64(median), ShouldEqual, 2.5)
		})
	})

	Convey("Given an empty stream", testingTB, func() {
		median := NewMedian(nil).Observe()

		Convey("It should return zero", func() {
			So(float64(median), ShouldEqual, 0)
		})
	})

	Convey("Given a weighted stream", testingTB, func() {
		median := NewMedian(nomagique.Numbers(1, 1, 1, 3)).Observe(
			nomagique.Numbers(1, 2, 3, 100)...,
		)

		Convey("It should follow the empirical weighted median", func() {
			So(float64(median), ShouldEqual, 3)
		})
	})

	Convey("Given mismatched weights", testingTB, func() {
		median := NewMedian(nomagique.Numbers(1, 1)).Observe(
			nomagique.Numbers(1, 2, 3)...,
		)

		Convey("It should return zero", func() {
			So(float64(median), ShouldEqual, 0)
		})
	})

	Convey("Given a non-finite sample", testingTB, func() {
		median := NewMedian(nomagique.Numbers(1, 1, 1)).Observe(
			core.Float64(1), core.Float64(math.NaN()), core.Float64(3),
		)

		Convey("It should return NaN", func() {
			So(math.IsNaN(float64(median)), ShouldBeTrue)
		})
	})
}

func BenchmarkMedian_Observe(testingTB *testing.B) {
	inputs := nomagique.Numbers(1, 2, 3, 4, 5, 6, 7, 8)
	median := NewMedian(nil)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = median.Observe(inputs...)
	}
}

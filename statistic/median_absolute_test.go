package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique"
)

func TestMedianAbsolute_Observe(testingTB *testing.T) {
	Convey("Given signed samples", testingTB, func() {
		value := NewMedianAbsolute(nil).Observe(
			nomagique.Numbers(-3, 1, -4, 2)...,
		)

		Convey("It should return the median of absolute values", func() {
			So(value, ShouldAlmostEqual, 2.5, 1e-9)
		})
	})
}

func BenchmarkMedianAbsolute_Observe(testingTB *testing.B) {
	inputs := nomagique.Numbers(-3, 1, -4, 2, 5, -1)
	medianAbsolute := NewMedianAbsolute(nil)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = medianAbsolute.Observe(inputs...)
	}
}

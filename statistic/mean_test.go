package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique"
)

func TestMean_Observe(testingTB *testing.T) {
	Convey("Given an unweighted stream", testingTB, func() {
		mean := NewMean(nil).Observe(nomagique.Numbers(1, 2, 3, 4)...)

		Convey("It should return the arithmetic mean", func() {
			So(float64(mean), ShouldEqual, 2.5)
		})
	})
}

func BenchmarkMean_Observe(testingTB *testing.B) {
	inputs := nomagique.Numbers(1, 2, 3, 4, 5, 6, 7, 8)
	mean := NewMean(nil)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = mean.Observe(inputs...)
	}
}

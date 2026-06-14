package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique"
)

func TestMin_Observe(testingTB *testing.T) {
	Convey("Given a stream", testingTB, func() {
		value := NewMin().Observe(nomagique.Numbers(3, 1, 4, 2)...)

		Convey("It should return the smallest value", func() {
			So(value, ShouldEqual, 1)
		})
	})
}

func BenchmarkMin_Observe(testingTB *testing.B) {
	inputs := nomagique.Numbers(3, 1, 4, 2, 5, 0, 8)
	min := NewMin()

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = min.Observe(inputs...)
	}
}

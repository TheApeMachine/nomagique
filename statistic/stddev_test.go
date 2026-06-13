package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique"
)

func TestStdDev_Observe(testingTB *testing.T) {
	Convey("Given a spread stream", testingTB, func() {
		value := NewStdDev(nil).Observe(nomagique.Numbers(1, 2, 3, 4, 5)...)

		Convey("It should return positive dispersion", func() {
			So(float64(value), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkStdDev_Observe(testingTB *testing.B) {
	inputs := nomagique.Numbers(1, 2, 3, 4, 5, 6, 7, 8)
	stdDev := NewStdDev(nil)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = stdDev.Observe(inputs...)
	}
}

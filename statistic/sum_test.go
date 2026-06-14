package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique"
)

func TestSum_Observe(testingTB *testing.T) {
	Convey("Given a batch of samples", testingTB, func() {
		value := NewSum().Observe(nomagique.Numbers(1.2, 0.8, 3.0)...)

		Convey("It should return the total", func() {
			So(value, ShouldAlmostEqual, 5.0, 1e-9)
		})
	})
}

func BenchmarkSum_Observe(testingTB *testing.B) {
	inputs := nomagique.Numbers(1.2, 0.8, 3.0, 2.0, 0.5)
	sumStage := NewSum()

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = sumStage.Observe(inputs...)
	}
}

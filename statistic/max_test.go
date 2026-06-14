package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique"
)

func TestMax_Observe(testingTB *testing.T) {
	Convey("Given a batch of samples", testingTB, func() {
		value := NewMax().Observe(nomagique.Numbers(3, 1, 4, 2)...)

		Convey("It should return the largest value", func() {
			So(value, ShouldEqual, 4)
		})
	})
}

func BenchmarkMax_Observe(testingTB *testing.B) {
	inputs := nomagique.Numbers(3, 1, 4, 2, 5, 0, 8)
	maxStage := NewMax()

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = maxStage.Observe(inputs...)
	}
}

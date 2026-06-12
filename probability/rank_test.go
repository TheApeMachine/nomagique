package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestRank(testingTB *testing.T) {
	Convey("Given Rank constructor", testingTB, func() {
		empirical := Rank()

		Convey("It should return a usable dynamic", func() {
			So(empirical, ShouldNotBeNil)
		})
	})
}

func TestEmpiricalRank_Observe(testingTB *testing.T) {
	Convey("Given empirical rank", testingTB, func() {
		empirical := Rank()
		empirical.Observe(core.Float64(10))
		value := empirical.Observe(core.Float64(5))

		Convey("It should return a probability in the unit interval", func() {
			So(float64(value), ShouldBeGreaterThan, 0)
			So(float64(value), ShouldBeLessThan, 1)
		})
	})
}

func BenchmarkRank_Observe(testingTB *testing.B) {
	empirical := Rank()
	empirical.Observe(core.Float64(10))

	for testingTB.Loop() {
		empirical.Observe(core.Float64(10.5))
	}
}

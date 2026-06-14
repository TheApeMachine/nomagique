package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestVariance(testingTB *testing.T) {
	Convey("Given Variance constructor", testingTB, func() {
		spread := Variance()

		Convey("It should return a usable dynamic", func() {
			So(spread, ShouldNotBeNil)
		})
	})
}

func TestSpread_Observe(testingTB *testing.T) {
	Convey("Given a fresh variance dynamic", testingTB, func() {
		spread := Variance()
		value := spread.Observe(core.Float64(10))

		Convey("It should bootstrap from the first sample", func() {
			So(value, ShouldEqual, core.Float64(0))
		})
	})
}

func TestSpread_Reset(testingTB *testing.T) {
	Convey("Given an observed variance dynamic", testingTB, func() {
		spread := Variance()
		_ = spread.Observe(core.Float64(10))

		Convey("When reset", func() {
			So(spread.Reset(), ShouldBeNil)
			value := spread.Observe(core.Float64(20))

			Convey("It should bootstrap again", func() {
				So(value, ShouldEqual, core.Float64(0))
			})
		})
	})
}

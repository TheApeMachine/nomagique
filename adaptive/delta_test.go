package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestDelta(testingTB *testing.T) {
	Convey("Given Delta constructor", testingTB, func() {
		normalized := Delta()

		Convey("It should return a usable dynamic", func() {
			So(normalized, ShouldNotBeNil)
		})
	})
}

func TestNormalized_Observe(testingTB *testing.T) {
	Convey("Given a fresh delta dynamic", testingTB, func() {
		normalized := Delta()

		Convey("When bootstrapping", func() {
			value := normalized.Observe(core.Float64(10))

			Convey("It should return zero delta", func() {
				So(value, ShouldEqual, core.Float64(0))
			})
		})
	})
}

func TestNormalized_Reset(testingTB *testing.T) {
	Convey("Given an observed delta dynamic", testingTB, func() {
		normalized := Delta()
		_ = normalized.Observe(core.Float64(10))
		_ = normalized.Observe(core.Float64(20))

		Convey("When reset", func() {
			So(normalized.Reset(), ShouldBeNil)
			value := normalized.Observe(core.Float64(30))

			Convey("It should bootstrap again", func() {
				So(value, ShouldEqual, core.Float64(0))
			})
		})
	})
}

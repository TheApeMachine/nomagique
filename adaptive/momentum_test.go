package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestMomentum(testingTB *testing.T) {
	Convey("Given Momentum constructor", testingTB, func() {
		inertia := Momentum()

		Convey("It should return a usable dynamic", func() {
			So(inertia, ShouldNotBeNil)
		})
	})
}

func TestInertia_Observe(testingTB *testing.T) {
	Convey("Given a fresh momentum dynamic", testingTB, func() {
		inertia := Momentum()
		value := inertia.Observe(core.Float64(10))

		Convey("It should return zero on bootstrap", func() {
			So(value, ShouldEqual, core.Float64(0))
		})
	})
}

func TestInertia_Reset(testingTB *testing.T) {
	Convey("Given an observed momentum dynamic", testingTB, func() {
		inertia := Momentum()
		_ = inertia.Observe(core.Float64(10))

		Convey("When reset", func() {
			So(inertia.Reset(), ShouldBeNil)
			value := inertia.Observe(core.Float64(20))

			Convey("It should bootstrap again", func() {
				So(value, ShouldEqual, core.Float64(0))
			})
		})
	})
}

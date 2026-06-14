package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestAccumulator(testingTB *testing.T) {
	Convey("Given Accumulator constructor", testingTB, func() {
		integrator := Accumulator()

		Convey("It should return a usable dynamic", func() {
			So(integrator, ShouldNotBeNil)
		})
	})
}

func TestIntegrator_Observe(testingTB *testing.T) {
	Convey("Given a fresh integrator", testingTB, func() {
		integrator := Accumulator()
		first := integrator.Observe(core.Float64(3))
		second := integrator.Observe(core.Float64(-1))

		Convey("It should integrate signed samples", func() {
			So(first, ShouldEqual, core.Float64(3))
			So(second, ShouldEqual, core.Float64(2))
		})
	})
}

func TestIntegrator_Reset(testingTB *testing.T) {
	Convey("Given an observed integrator", testingTB, func() {
		integrator := Accumulator()
		_ = integrator.Observe(core.Float64(5))

		Convey("When reset", func() {
			So(integrator.Reset(), ShouldBeNil)
			value := integrator.Observe(core.Float64(2))

			Convey("It should bootstrap again", func() {
				So(value, ShouldEqual, core.Float64(2))
			})
		})
	})
}

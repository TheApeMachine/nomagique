package geometry

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestCoupling(testingTB *testing.T) {
	Convey("Given Coupling constructor", testingTB, func() {
		phaseCoupling := Coupling()

		Convey("It should return a usable dynamic", func() {
			So(phaseCoupling, ShouldNotBeNil)
		})
	})
}

func TestPhaseCoupling_Observe(testingTB *testing.T) {
	Convey("Given co-moving growth", testingTB, func() {
		phaseCoupling := Coupling()
		value := phaseCoupling.Observe(core.Float64(2), core.Float64(2))

		Convey("It should return positive coupling", func() {
			So(float64(value), ShouldAlmostEqual, 1, 1e-9)
		})
	})

	Convey("Given opposing growth", testingTB, func() {
		phaseCoupling := Coupling()
		value := phaseCoupling.Observe(core.Float64(2), core.Float64(-2))

		Convey("It should return negative coupling", func() {
			So(float64(value), ShouldAlmostEqual, -1, 1e-9)
		})
	})
}

func BenchmarkCoupling_Observe(testingTB *testing.B) {
	phaseCoupling := Coupling()

	for testingTB.Loop() {
		phaseCoupling.Observe(core.Float64(1.7), core.Float64(-0.9))
	}
}

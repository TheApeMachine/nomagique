package geometry

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestVelocity(testingTB *testing.T) {
	Convey("Given Velocity constructor", testingTB, func() {
		phaseVelocity := Velocity()

		Convey("It should return a usable dynamic", func() {
			So(phaseVelocity, ShouldNotBeNil)
		})
	})
}

func TestPhaseVelocity_Observe(testingTB *testing.T) {
	Convey("Given phase velocity", testingTB, func() {
		phaseVelocity := Velocity()
		phaseVelocity.Observe(core.Float64(1))
		value := phaseVelocity.Observe(core.Float64(1.5))

		Convey("It should return the velocity", func() {
			So(float64(value), ShouldAlmostEqual, 0.5, 1e-12)
		})
	})
}

func BenchmarkVelocity_Observe(testingTB *testing.B) {
	phaseVelocity := Velocity()
	phaseVelocity.Observe(core.Float64(1))

	for testingTB.Loop() {
		phaseVelocity.Observe(core.Float64(1.5))
	}
}

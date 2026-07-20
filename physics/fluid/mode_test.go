package fluid

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewModePhysics(t *testing.T) {
	Convey("Given explicit natural-unit bath properties", t, func() {
		constants := PhysicalConstants{
			Source:  "natural",
			G:       1,
			KB:      1,
			SigmaSB: 1,
			HBar:    1,
		}

		physics, err := NewModePhysics(2, 0.25, constants, 0.2, 1)

		Convey("It should derive every scale from the physical inputs", func() {
			So(err, ShouldBeNil)
			So(physics.ThermalFrequency, ShouldAlmostEqual, 2.0)
			So(physics.CoherenceTime, ShouldAlmostEqual, 0.5)
			So(physics.ThermalAmplitude, ShouldAlmostEqual, math.Sqrt(2))
			So(physics.BandwidthMinimum, ShouldAlmostEqual, 2.0)
			So(physics.BandwidthMaximum, ShouldAlmostEqual, 2.0)
			So(physics.DecayRate, ShouldAlmostEqual, 0.4)
			So(physics.DecayFactor, ShouldAlmostEqual, math.Exp(-0.1))
			So(physics.CoherenceSteps, ShouldEqual, 2)
		})

		Convey("It should classify visibility against the thermal floor", func() {
			floor := physics.NoiseFloor(1, constants, 1)
			So(physics.Visible(floor, 1, constants, 1), ShouldBeFalse)
			So(physics.Visible(2*floor, 1, constants, 1), ShouldBeTrue)
			So(physics.VisibilityRatio(2*floor, 1, constants, 1), ShouldAlmostEqual, 2.0)
		})
	})
}

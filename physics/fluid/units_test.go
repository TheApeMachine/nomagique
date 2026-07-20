package fluid

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestOmegaNaturalUnitSystem(t *testing.T) {
	Convey("Given the Sensorium dry-air material declaration", t, func() {
		constants := DefaultSIConstants()
		units, err := OmegaNaturalUnitSystem(1.4, 0.02897, 1, constants)

		Convey("It should derive finite natural base units", func() {
			So(err, ShouldBeNil)
			So(units.LengthMetres, ShouldEqual, 1.0)
			So(units.MassKilograms, ShouldBeGreaterThan, 0)
			So(units.TimeSeconds, ShouldBeGreaterThan, 0)
			So(units.TemperatureKelvin, ShouldBeGreaterThan, 0)
		})

		Convey("It should make hbar, k_B, and c_v unity", func() {
			converted, conversionErr := ConstantsFromSI(units, constants)
			So(conversionErr, ShouldBeNil)
			gasConstant, gasErr := units.GasSpecificConstant(0.02897, constants)
			So(gasErr, ShouldBeNil)
			So(math.Abs(converted.HBar-1), ShouldBeLessThan, 1e-12)
			So(math.Abs(converted.KB-1), ShouldBeLessThan, 1e-12)
			So(math.Abs(gasConstant/(1.4-1)-1), ShouldBeLessThan, 1e-12)
		})
	})
}

func TestPhysicalConstantsThermalAmplitude(t *testing.T) {
	Convey("Given a natural-unit thermal bath", t, func() {
		constants := PhysicalConstants{
			Source:  "codata_si_derived",
			G:       1,
			KB:      1,
			SigmaSB: 1,
			HBar:    1,
		}

		Convey("It should retain zero-point fluctuations at zero temperature", func() {
			amplitude := constants.ThermalAmplitude(2, 0, 1)
			So(amplitude, ShouldAlmostEqual, 0.5)
		})

		Convey("It should raise the noise floor with temperature", func() {
			cold := constants.ThermalAmplitude(2, 0, 1)
			hot := constants.ThermalAmplitude(2, 4, 1)
			So(hot, ShouldBeGreaterThan, cold)
			So(constants.ModeVisibilityRatio(hot*2, 2, 4, 1), ShouldAlmostEqual, 2.0)
		})
	})
}

package manifold

import (
	"math"
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func TestApplyDerivedGasParams(t *testing.T) {
	convey.Convey("Given production grid dimensions", t, func() {
		config := productionTestConfig()

		convey.Convey("It should use per-carrier envelope density for gas primitives", func() {
			convey.So(config.GasEnvelopeRhoMin, convey.ShouldAlmostEqual, config.RhoMin/float64(config.MaxModes), 1e-9)
			convey.So(config.GasPMin, convey.ShouldAlmostEqual, (config.Gamma-1.0)*config.GasEnvelopeRhoMin*config.CellVolume(), 1e-9)
		})

		convey.Convey("It should satisfy the von Neumann diffusion CFL bound", func() {
			convey.So(config.DiffusionCFL(), convey.ShouldBeLessThan, 1.0/6.0)
			convey.So(config.Validate(), convey.ShouldBeNil)
		})

		convey.Convey("It should place typical deposits inside the envelope scale", func() {
			typicalDeposit := config.RhoMin / float64(config.MaxModes)
			convey.So(typicalDeposit, convey.ShouldBeGreaterThan, config.GasEnvelopeRhoMin*0.5)
		})
	})
}

func TestValidateRejectsUnstableDiffusion(t *testing.T) {
	convey.Convey("Given an explicit diffusion CFL violation", t, func() {
		config := productionTestConfig()
		config.KThermal = config.RhoMin / config.DeltaT

		convey.Convey("It should fail validation", func() {
			convey.So(config.DiffusionCFL(), convey.ShouldBeGreaterThan, 1.0/6.0)
			convey.So(config.Validate(), convey.ShouldNotBeNil)
		})
	})
}

func TestConfigRuntimeControls(t *testing.T) {
	convey.Convey("Given a validated manifold config", t, func() {
		config := productionTestConfig()

		convey.Convey("It should expose the integration controls used by the solver", func() {
			controls := config.RuntimeControls()

			convey.So(controls.DeltaT, convey.ShouldEqual, config.DeltaT)
			convey.So(controls.MetabolicRate, convey.ShouldEqual, config.MetabolicRate())
			convey.So(controls.Validate(), convey.ShouldBeNil)
		})
	})
}

func TestRuntimeControlsValidate(t *testing.T) {
	convey.Convey("Given invalid runtime controls", t, func() {
		controls := productionTestConfig().RuntimeControls()
		controls.TopdownPhaseScale = math.Inf(1)

		convey.Convey("It should reject non-finite prior values", func() {
			convey.So(controls.Validate(), convey.ShouldNotBeNil)
		})
	})
}

func productionTestConfig() Config {
	tickSize := 0.01
	halfWidth := 32
	gamma := 5.0 / 3.0
	deltaT := 0.1

	config := Config{
		GridX:    32,
		GridY:    3,
		GridZ:    16,
		DomainX:  float64(halfWidth*2+1) * tickSize,
		DomainY:  3,
		DomainZ:  16,
		DeltaT:   deltaT,
		Gamma:    gamma,
		MaxModes: 128,
	}

	ApplyDerivedGasParams(&config)

	return config
}

func BenchmarkDiffusionCFL(b *testing.B) {
	config := productionTestConfig()

	for b.Loop() {
		if math.IsNaN(config.DiffusionCFL()) {
			b.Fatal("diffusion CFL is NaN")
		}
	}
}

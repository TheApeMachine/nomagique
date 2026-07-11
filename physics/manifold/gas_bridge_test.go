//go:build darwin && cgo

package manifold

import (
	"math"
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func marketGasTestConfig() Config {
	config := Config{
		GridX:    8,
		GridY:    1,
		GridZ:    8,
		DomainX:  0.16,
		DomainY:  1,
		DomainZ:  8,
		DeltaT:   0.01,
		Gamma:    5.0 / 3.0,
		MaxModes: 4,
	}
	ApplyDerivedGasParams(&config)
	DefaultMarketGasBoundaries().Apply(&config)

	return config
}

func testOscillator() Oscillator {
	return Oscillator{
		Phase:     0.5,
		Omega:     6.28,
		Amplitude: 0.2,
		PosX:      0.4,
		PosY:      0,
		PosZ:      1.2,
		Heat:      0.2,
		VelX:      0.4,
	}
}

func seedSolverForStep(t *testing.T, solver *Solver, config Config) {
	t.Helper()

	convey.So(solver.ResetDeposits(), convey.ShouldBeNil)
	convey.So(solver.DepositCell(config.GridX/2, 0, config.GridZ/2, 0.05, 0, 0, 0, 0.05), convey.ShouldBeNil)
	convey.So(solver.SetOscillators([]Oscillator{testOscillator()}), convey.ShouldBeNil)
}

func TestGasSourceInjectionReconcilesDeltas(t *testing.T) {
	convey.Convey("Given an exact conserved-state source", t, func() {
		config := marketGasTestConfig()
		solver := NewSolver(config)

		convey.So(solver, convey.ShouldNotBeNil)
		defer solver.Close()

		convey.So(solver.ResetDeposits(), convey.ShouldBeNil)
		convey.So(solver.SourceCell(2, 0, 3, 0.2, -0.1, 0.05, 0.04, 0.03), convey.ShouldBeNil)
		convey.So(solver.ApplySources(), convey.ShouldBeNil)

		rho, momX, momY, momZ, eInt, err := solver.ReadCell(2, 0, 3)

		convey.So(err, convey.ShouldBeNil)
		convey.So(rho, convey.ShouldAlmostEqual, 0.04, 1e-6)
		convey.So(momX, convey.ShouldAlmostEqual, 0.2, 1e-6)
		convey.So(momY, convey.ShouldAlmostEqual, -0.1, 1e-6)
		convey.So(momZ, convey.ShouldAlmostEqual, 0.05, 1e-6)
		convey.So(eInt, convey.ShouldAlmostEqual, 0.03, 1e-6)
	})
}

func TestGasSourceRemovalReachesVacuum(t *testing.T) {
	convey.Convey("Given a deposited cell population", t, func() {
		config := marketGasTestConfig()
		solver := NewSolver(config)

		convey.So(solver, convey.ShouldNotBeNil)
		defer solver.Close()

		convey.So(solver.ResetDeposits(), convey.ShouldBeNil)
		convey.So(solver.DepositCell(1, 0, 1, 0.05, 0.1, 0, 0, 0.05), convey.ShouldBeNil)
		convey.So(solver.SourceCell(1, 0, 1, -0.1, 0, 0, -0.05, -0.05), convey.ShouldBeNil)
		convey.So(solver.ApplySources(), convey.ShouldBeNil)

		rho, momX, _, _, eInt, err := solver.ReadCell(1, 0, 1)

		convey.So(err, convey.ShouldBeNil)
		convey.So(rho, convey.ShouldAlmostEqual, 0, 1e-6)
		convey.So(momX, convey.ShouldAlmostEqual, 0, 1e-6)
		convey.So(eInt, convey.ShouldAlmostEqual, 0, 1e-6)
	})
}

func TestGasExcessiveRemovalIsInadmissible(t *testing.T) {
	convey.Convey("Given a source that removes more than the conserved state", t, func() {
		config := marketGasTestConfig()
		solver := NewSolver(config)

		convey.So(solver, convey.ShouldNotBeNil)
		defer solver.Close()

		convey.So(solver.ResetDeposits(), convey.ShouldBeNil)
		convey.So(solver.DepositCell(1, 0, 1, 0.05, 0, 0, 0, 0.05), convey.ShouldBeNil)
		convey.So(solver.SourceCell(1, 0, 1, 0, 0, 0, -1.0, -1.0), convey.ShouldBeNil)

		convey.So(solver.ApplySources(), convey.ShouldNotBeNil)
	})
}

func TestGasInvalidSourceBufferIsRejected(t *testing.T) {
	convey.Convey("Given a non-finite source delta", t, func() {
		config := marketGasTestConfig()
		solver := NewSolver(config)

		convey.So(solver, convey.ShouldNotBeNil)
		defer solver.Close()

		convey.So(solver.ResetDeposits(), convey.ShouldBeNil)
		convey.So(solver.SourceCell(1, 0, 1, math.NaN(), 0, 0, 0.01, 0), convey.ShouldNotBeNil)
	})
}

func TestGasOutflowPulseDoesNotWrap(t *testing.T) {
	convey.Convey("Given an outflow pulse at the high price face", t, func() {
		config := marketGasTestConfig()
		solver := NewSolver(config)

		convey.So(solver, convey.ShouldNotBeNil)
		defer solver.Close()

		convey.So(solver.ResetDeposits(), convey.ShouldBeNil)
		convey.So(
			solver.DepositCell(config.GridX-1, 0, config.GridZ/2, 0.2, 0.5, 0, 0, 0.2),
			convey.ShouldBeNil,
		)
		convey.So(solver.SetOscillators([]Oscillator{testOscillator()}), convey.ShouldBeNil)

		_, stepErr := solver.Step()

		convey.So(stepErr, convey.ShouldBeNil)

		lowRho, _, _, _, _, err := solver.ReadCell(0, 0, config.GridZ/2)

		convey.So(err, convey.ShouldBeNil)
		convey.So(lowRho, convey.ShouldBeLessThan, 0.05)
	})
}

func TestGasOutflowBoundaryAdmitsNoIncomingMass(t *testing.T) {
	convey.Convey("Given an isolated outflow boundary cell", t, func() {
		config := marketGasTestConfig()
		solver := NewSolver(config)

		convey.So(solver, convey.ShouldNotBeNil)
		defer solver.Close()

		convey.So(solver.ResetDeposits(), convey.ShouldBeNil)
		convey.So(solver.DepositCell(0, 0, 1, 0.1, 0.5, 0, 0, 0.1), convey.ShouldBeNil)
		convey.So(solver.SetOscillators([]Oscillator{testOscillator()}), convey.ShouldBeNil)

		_, stepErr := solver.Step()

		convey.So(stepErr, convey.ShouldBeNil)

		boundaryRho, _, _, _, _, err := solver.ReadCell(0, 0, 1)

		convey.So(err, convey.ShouldBeNil)
		convey.So(boundaryRho, convey.ShouldBeLessThanOrEqualTo, 0.11)
	})
}

func TestGasReflectingBoundaryPreservesBoundaryMass(t *testing.T) {
	convey.Convey("Given reflecting versus outflow low-price faces", t, func() {
		outflowConfig := marketGasTestConfig()
		reflectConfig := marketGasTestConfig()
		reflectConfig.BoundaryXLow = GasBoundaryReflecting

		runBoundaryStep := func(config Config) float64 {
			solver := NewSolver(config)
			convey.So(solver, convey.ShouldNotBeNil)
			defer solver.Close()

			convey.So(solver.ResetDeposits(), convey.ShouldBeNil)
			convey.So(solver.DepositCell(0, 0, config.GridZ/2, 0.1, -0.4, 0, 0, 0.1), convey.ShouldBeNil)
			convey.So(solver.SetOscillators([]Oscillator{testOscillator()}), convey.ShouldBeNil)
			_, stepErr := solver.Step()
			convey.So(stepErr, convey.ShouldBeNil)

			rho, _, _, _, _, err := solver.ReadCell(0, 0, config.GridZ/2)
			convey.So(err, convey.ShouldBeNil)

			return rho
		}

		outflowRho := runBoundaryStep(outflowConfig)
		reflectingRho := runBoundaryStep(reflectConfig)

		convey.So(reflectingRho, convey.ShouldBeGreaterThan, outflowRho)
	})
}

func TestGasUnequalAxisSpacingSteps(t *testing.T) {
	convey.Convey("Given unequal dx, dy, and dz", t, func() {
		config := Config{
			GridX:    8,
			GridY:    2,
			GridZ:    8,
			DomainX:  0.16,
			DomainY:  4,
			DomainZ:  8,
			DeltaT:   0.01,
			Gamma:    5.0 / 3.0,
			MaxModes: 4,
		}
		ApplyDerivedGasParams(&config)
		DefaultMarketGasBoundaries().Apply(&config)

		solver := NewSolver(config)

		convey.So(solver, convey.ShouldNotBeNil)
		defer solver.Close()

		seedSolverForStep(t, solver, config)

		reading, stepErr := solver.Step()

		convey.So(stepErr, convey.ShouldBeNil)
		convey.So(math.IsNaN(reading.PressureGradNorm), convey.ShouldBeFalse)
	})
}

func TestGasInvalidAdvectiveCFLPoisonsStep(t *testing.T) {
	convey.Convey("Given an advective CFL violation", t, func() {
		config := marketGasTestConfig()
		solver := NewSolver(config)

		convey.So(solver, convey.ShouldNotBeNil)
		defer solver.Close()

		convey.So(solver.ResetDeposits(), convey.ShouldBeNil)
		convey.So(solver.DepositCell(2, 0, 2, 0.01, 20, 0, 0, 0.01), convey.ShouldBeNil)
		convey.So(solver.SetControls(RuntimeControls{DeltaT: 10, MetabolicRate: 0.1}), convey.ShouldBeNil)

		convey.So(solver.RunGasTransport(), convey.ShouldNotBeNil)
	})
}

func TestGasInvalidDiffusiveCFLPoisonsStep(t *testing.T) {
	convey.Convey("Given a diffusive CFL violation", t, func() {
		config := marketGasTestConfig()
		config.KThermal = config.GasEnvelopeRhoMin * config.CV * 1000
		solver := NewSolver(config)

		convey.So(solver, convey.ShouldNotBeNil)
		defer solver.Close()

		convey.So(solver.ResetDeposits(), convey.ShouldBeNil)
		convey.So(solver.DepositCell(2, 0, 2, 0.05, 0, 0, 0, 0.05), convey.ShouldBeNil)

		convey.So(solver.RunGasTransport(), convey.ShouldNotBeNil)
	})
}

func TestGPEPeriodicBehaviorRemainsFinite(t *testing.T) {
	convey.Convey("Given market gas boundaries with GPE carriers", t, func() {
		config := marketGasTestConfig()
		solver := NewSolver(config)

		convey.So(solver, convey.ShouldNotBeNil)
		defer solver.Close()

		seedSolverForStep(t, solver, config)

		reading, stepErr := solver.Step()

		convey.So(stepErr, convey.ShouldBeNil)
		convey.So(reading.CoherenceMag2, convey.ShouldBeGreaterThan, 0)
		convey.So(math.IsInf(reading.CoherenceMag2, 0), convey.ShouldBeFalse)
	})
}

func BenchmarkGasApplySources(b *testing.B) {
	config := marketGasTestConfig()
	solver := NewSolver(config)

	if solver == nil {
		b.Fatal("solver was not created")
	}

	defer solver.Close()

	_ = solver.ResetDeposits()
	_ = solver.SourceCell(2, 0, 3, 0.1, 0, 0, 0.05, 0.05)

	b.ReportAllocs()

	for b.Loop() {
		_ = solver.ResetDeposits()
		_ = solver.SourceCell(2, 0, 3, 0.1, 0, 0, 0.05, 0.05)
		_ = solver.ApplySources()
	}
}

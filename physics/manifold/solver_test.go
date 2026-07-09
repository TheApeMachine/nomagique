//go:build darwin && cgo

package manifold

import (
	"fmt"
	"math"
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func smallTestConfig() Config {
	config := Config{
		GridX:    8,
		GridY:    1,
		GridZ:    8,
		DomainX:  0.16,
		DomainY:  1,
		DomainZ:  8,
		DeltaT:   0.1,
		Gamma:    5.0 / 3.0,
		MaxModes: 4,
	}
	ApplyDerivedGasParams(&config)

	return config
}

func TestSolverStep(t *testing.T) {
	convey.Convey("Given a Metal manifold solver", t, func() {
		config := smallTestConfig()

		solver := NewSolver(config)

		convey.Convey("It should accept deposits, oscillators, and return finite readings", func() {
			convey.So(solver, convey.ShouldNotBeNil)

			defer solver.Close()

			convey.So(solver.ResetDeposits(), convey.ShouldBeNil)

			for cellX := uint32(0); cellX < config.GridX; cellX++ {
				for cellZ := uint32(0); cellZ < config.GridZ; cellZ++ {
					convey.So(solver.DepositCell(cellX, 0, cellZ, 0.05, 0, 0, 0, 0.05), convey.ShouldBeNil)
				}
			}

			convey.So(solver.SetOscillators([]Oscillator{{
				Phase:     0.5,
				Omega:     6.28,
				Amplitude: 0.2,
				PosX:      0.4,
				PosY:      0,
				PosZ:      1.2,
				Heat:      0.2,
				VelX:      0.4,
			}}), convey.ShouldBeNil)

			reading, stepErr := solver.Step()

			convey.So(stepErr, convey.ShouldBeNil)
			convey.So(math.IsNaN(reading.PressureGradNorm), convey.ShouldBeFalse)
			convey.So(math.IsInf(reading.CoherenceMag2, 0), convey.ShouldBeFalse)
			convey.So(reading.CoherenceMag2, convey.ShouldBeGreaterThan, 0)

			rho, rhoErr := solver.ReadRhoProjection()

			convey.So(rhoErr, convey.ShouldBeNil)
			convey.So(len(rho), convey.ShouldEqual, int(config.GridZ))
			convey.So(len(rho[0]), convey.ShouldEqual, int(config.GridX))
		})
	})
}

func TestSolverSetControls(t *testing.T) {
	convey.Convey("Given a Metal manifold solver", t, func() {
		config := smallTestConfig()
		solver := NewSolver(config)

		convey.Convey("It should accept validated runtime controls before stepping", func() {
			convey.So(solver, convey.ShouldNotBeNil)

			defer solver.Close()

			controls := config.RuntimeControls()
			controls.DeltaT = config.DeltaT * 0.5
			controls.MetabolicRate = 1 / controls.DeltaT
			controls.TopdownPhaseScale = 0.25
			controls.TopdownEnergyScale = 0.25

			convey.So(solver.SetControls(controls), convey.ShouldBeNil)

			controls.DeltaT = 0
			convey.So(solver.SetControls(controls), convey.ShouldNotBeNil)
		})
	})
}

func TestReadProjectionReading(t *testing.T) {
	convey.Convey("Given a deposited rho lattice", t, func() {
		config := smallTestConfig()

		solver := NewSolver(config)

		convey.Convey("It should derive bulk observables from the rho projection", func() {
			convey.So(solver, convey.ShouldNotBeNil)

			defer solver.Close()

			convey.So(solver.ResetDeposits(), convey.ShouldBeNil)
			convey.So(solver.DepositCell(4, 0, 4, 1, 0, 0, 0, config.CV), convey.ShouldBeNil)
			convey.So(solver.SetOscillators([]Oscillator{{
				Amplitude: 0.2,
				Heat:      0.2,
			}}), convey.ShouldBeNil)
			_, stepErr := solver.Step()

			convey.So(stepErr, convey.ShouldBeNil)

			reading, projectionErr := solver.ReadProjectionReading()

			convey.So(projectionErr, convey.ShouldBeNil)
			convey.So(reading.PressureGradNorm, convey.ShouldBeGreaterThan, 0)
			convey.So(reading.Divergence, convey.ShouldBeGreaterThan, 0)
			convey.So(reading.ViscosityProxy, convey.ShouldBeGreaterThan, 0)
		})
	})
}

func TestReadOscillators(t *testing.T) {
	convey.Convey("Given a stepped solver with oscillators", t, func() {
		config := smallTestConfig()

		solver := NewSolver(config)

		convey.Convey("It should read post-step particle state from Metal", func() {
			convey.So(solver, convey.ShouldNotBeNil)

			defer solver.Close()

			convey.So(solver.ResetDeposits(), convey.ShouldBeNil)
			convey.So(solver.DepositCell(4, 0, 4, 0.5, 0, 0, 0, 0.5), convey.ShouldBeNil)
			convey.So(solver.SetOscillators([]Oscillator{{
				Phase:     0.5,
				Omega:     6.28,
				Amplitude: 0.2,
				PosX:      0.09,
				PosY:      0,
				PosZ:      4.5,
				Heat:      0.2,
				VelX:      0.4,
			}}), convey.ShouldBeNil)

			_, stepErr := solver.Step()

			convey.So(stepErr, convey.ShouldBeNil)

			oscillators, err := solver.ReadOscillators(1)

			convey.So(err, convey.ShouldBeNil)
			convey.So(len(oscillators), convey.ShouldEqual, 1)
			convey.So(oscillators[0].Heat, convey.ShouldBeGreaterThan, 0)
			convey.So(math.IsNaN(oscillators[0].PosX), convey.ShouldBeFalse)
			convey.So(math.IsNaN(oscillators[0].PosY), convey.ShouldBeFalse)
			convey.So(math.IsNaN(oscillators[0].PosZ), convey.ShouldBeFalse)
			convey.So(math.IsNaN(oscillators[0].VelX), convey.ShouldBeFalse)
			convey.So(math.IsNaN(oscillators[0].VelY), convey.ShouldBeFalse)
			convey.So(math.IsNaN(oscillators[0].VelZ), convey.ShouldBeFalse)
		})
	})
}

func TestReadOscillatorsDecisionLattice(t *testing.T) {
	convey.Convey("Given the decision manifold lattice", t, func() {
		config := Config{
			GridX:    8,
			GridY:    11,
			GridZ:    8,
			DomainX:  8,
			DomainY:  11,
			DomainZ:  8,
			DeltaT:   0.1,
			Gamma:    5.0 / 3.0,
			MaxModes: 11,
		}
		ApplyDerivedGasParams(&config)

		solver := NewSolver(config)

		convey.Convey("It should return finite post-step particle readback", func() {
			convey.So(solver, convey.ShouldNotBeNil)

			defer solver.Close()

			convey.So(solver.ResetDeposits(), convey.ShouldBeNil)

			clamps := []struct {
				cellY uint32
				rho   float64
				momX  float64
				momY  float64
				momZ  float64
			}{
				{cellY: 0, rho: 0.52, momX: 0.36, momZ: 0.36},
				{
					cellY: 1,
					rho:   0.5956521739130435,
					momX:  0.5956521739130435,
					momZ:  0.5956521739130435,
				},
				{
					cellY: 2,
					rho:   0.3883116883116883,
					momX:  0.325974025974026,
					momY:  0.03116883116883117,
					momZ:  0.35714285714285715,
				},
				{
					cellY: 4,
					rho:   0.42142857142857143,
					momX:  0.3,
					momZ:  0.3,
				},
			}
			oscillators := make([]Oscillator, 0, len(clamps))

			for _, clamp := range clamps {
				pressure := math.Abs(clamp.momX) + clamp.momY + clamp.momZ
				internalEnergy := pressure / (config.Gamma - 1)
				oscillators = append(oscillators, Oscillator{
					Phase:     math.Atan2(clamp.momX, pressure),
					Omega:     pressure,
					Amplitude: math.Sqrt(clamp.rho),
					PosX:      7,
					PosY:      float64(clamp.cellY),
					PosZ:      0,
					Heat:      internalEnergy,
					VelX:      clamp.momX,
					VelY:      clamp.momY,
					VelZ:      clamp.momZ,
				})
			}

			convey.So(solver.SetOscillators(oscillators), convey.ShouldBeNil)

			_, stepErr := solver.Step()

			convey.So(stepErr, convey.ShouldBeNil)

			readback, readErr := solver.ReadOscillators(len(oscillators))

			convey.So(readErr, convey.ShouldBeNil)

			for index, oscillator := range readback {
				values := map[string]float64{
					"phase": oscillator.Phase,
					"pos_x": oscillator.PosX,
					"pos_y": oscillator.PosY,
					"pos_z": oscillator.PosZ,
					"vel_x": oscillator.VelX,
					"vel_y": oscillator.VelY,
					"vel_z": oscillator.VelZ,
				}

				for name, value := range values {
					if math.IsNaN(value) || math.IsInf(value, 0) {
						t.Fatalf(
							"oscillator %d %s non-finite: %+v",
							index,
							name,
							readback,
						)
					}
				}
			}
		})
	})
}

func TestSolverWhaleParticleVelocity(t *testing.T) {
	convey.Convey("Given a Metal manifold solver", t, func() {
		config := smallTestConfig()

		solver := NewSolver(config)

		convey.Convey("It should step with whale particles carrying directional velocity", func() {
			defer solver.Close()

			convey.So(solver.ResetDeposits(), convey.ShouldBeNil)

			for cellX := uint32(0); cellX < config.GridX; cellX++ {
				for cellZ := uint32(0); cellZ < config.GridZ; cellZ++ {
					convey.So(solver.DepositCell(cellX, 0, cellZ, 0.05, 0, 0, 0, 0.05), convey.ShouldBeNil)
				}
			}

			convey.So(solver.SetOscillators([]Oscillator{{
				Phase:     0.5,
				Omega:     6.28,
				Amplitude: 0.2,
				PosX:      0.4,
				PosY:      0,
				PosZ:      1.2,
				Heat:      0.2,
				VelX:      0.4,
			}}), convey.ShouldBeNil)

			reading, stepErr := solver.Step()

			convey.So(stepErr, convey.ShouldBeNil)
			convey.So(math.IsNaN(reading.PressureGradNorm), convey.ShouldBeFalse)
			convey.So(math.IsInf(reading.CoherenceMag2, 0), convey.ShouldBeFalse)
			convey.So(reading.CoherenceMag2, convey.ShouldBeGreaterThan, 0)
		})
	})
}

func TestSolverProductionConfig(t *testing.T) {
	convey.Convey("Given production manifold grid dimensions", t, func() {
		config := Config{
			GridX:    32,
			GridY:    3,
			GridZ:    16,
			DomainX:  0.32,
			DomainY:  3,
			DomainZ:  16,
			DeltaT:   0.1,
			Gamma:    5.0 / 3.0,
			MaxModes: 32,
		}
		ApplyDerivedGasParams(&config)

		solver := NewSolver(config)

		convey.Convey("It should step with 32 oscillators on a 32x3x16 grid", func() {
			defer solver.Close()

			convey.So(solver.ResetDeposits(), convey.ShouldBeNil)

			for cellX := uint32(0); cellX < config.GridX; cellX++ {
				for cellY := uint32(0); cellY < config.GridY; cellY++ {
					for cellZ := uint32(0); cellZ < config.GridZ; cellZ++ {
						convey.So(solver.DepositCell(cellX, cellY, cellZ, 0.05, 0, 0, 0, 0.05), convey.ShouldBeNil)
					}
				}
			}

			oscillators := make([]Oscillator, config.MaxModes)

			for index := range oscillators {
				oscillators[index] = Oscillator{
					Phase:     float64(index) * 0.1,
					Omega:     6.28,
					Amplitude: 0.1,
					PosX:      float64(index % int(config.GridX)),
					PosY:      float64(index % int(config.GridY)),
					PosZ:      float64(index % int(config.GridZ)),
					Heat:      0.1,
				}
			}

			convey.So(solver.SetOscillators(oscillators), convey.ShouldBeNil)

			reading, stepErr := solver.Step()

			convey.So(stepErr, convey.ShouldBeNil)
			convey.So(math.IsNaN(reading.PressureGradNorm), convey.ShouldBeFalse)
			convey.So(math.IsInf(reading.CoherenceMag2, 0), convey.ShouldBeFalse)

			rho, rhoErr := solver.ReadRhoProjection()

			convey.So(rhoErr, convey.ShouldBeNil)
			convey.So(len(rho), convey.ShouldEqual, int(config.GridZ))
			convey.So(len(rho[0]), convey.ShouldEqual, int(config.GridX))
		})
	})
}

func TestSolverCarrierThreshold(t *testing.T) {
	config := productionTestConfig()
	deltaT := config.DeltaT

	for _, count := range []int{128} {
		t.Run(fmt.Sprintf("count=%d", count), func(t *testing.T) {
			solver := NewSolver(config)

			defer solver.Close()

			if err := solver.ResetDeposits(); err != nil {
				t.Fatal(err)
			}

			if err := solver.DepositCell(1, 0, 1, 0.05, 0, 0, 0, 0.05); err != nil {
				t.Fatal(err)
			}

			omega := 2 * math.Pi / deltaT
			osc := make([]Oscillator, count)

			for index := range osc {
				perCarrierEnergy := config.RhoMin / float64(count)
				osc[index] = Oscillator{
					Phase:     float64(index) * 0.1,
					Omega:     omega,
					Amplitude: math.Sqrt(perCarrierEnergy),
					Heat:      perCarrierEnergy,
					PosX:      1,
					PosY:      0,
					PosZ:      1,
				}
			}

			if err := solver.SetOscillators(osc); err != nil {
				t.Fatal(err)
			}

			if _, err := solver.Step(); err != nil {
				t.Fatal(err)
			}

			read, err := solver.ReadOscillators(count)

			if err != nil {
				t.Fatal(err)
			}

			if math.IsNaN(read[0].Phase) {
				t.Fatalf("phase NaN at count %d", count)
			}
		})
	}
}

func TestSolverProduction128Oscillators(t *testing.T) {
	convey.Convey("Given production rho_min and 128 startup oscillators", t, func() {
		config := productionTestConfig()
		carrierCount := 128

		solver := NewSolver(config)

		convey.Convey("It should return finite oscillator readback after step", func() {
			defer solver.Close()

			convey.So(solver.ResetDeposits(), convey.ShouldBeNil)
			convey.So(
				solver.DepositCell(1, 0, 1, 0.05, 0, 0, 0, 0.05),
				convey.ShouldBeNil,
			)

			omega := 2 * math.Pi / config.DeltaT
			oscillators := make([]Oscillator, carrierCount)

			for index := range oscillators {
				perCarrierEnergy := config.RhoMin / float64(carrierCount)
				oscillators[index] = Oscillator{
					Phase:     float64(index) * 0.1,
					Omega:     omega,
					Amplitude: math.Sqrt(perCarrierEnergy),
					PosX:      1,
					PosY:      0,
					PosZ:      1,
					Heat:      perCarrierEnergy,
				}
			}

			convey.So(solver.SetOscillators(oscillators), convey.ShouldBeNil)

			_, stepErr := solver.Step()

			convey.So(stepErr, convey.ShouldBeNil)

			readback, err := solver.ReadOscillators(len(oscillators))

			convey.So(err, convey.ShouldBeNil)
			convey.So(len(readback), convey.ShouldEqual, carrierCount)
			convey.So(math.IsNaN(readback[0].Phase), convey.ShouldBeFalse)
			convey.So(math.IsNaN(readback[0].Heat), convey.ShouldBeFalse)
			convey.So(math.IsNaN(readback[0].Amplitude), convey.ShouldBeFalse)
		})
	})
}

func TestSolverMultiSymbolDeposits(t *testing.T) {
	config := productionTestConfig()
	carrierCount := 128

	solver := NewSolver(config)

	defer solver.Close()

	if err := solver.ResetDeposits(); err != nil {
		t.Fatal(err)
	}

	for symbolIndex := 0; symbolIndex < carrierCount; symbolIndex++ {
		cellZ := uint32(symbolIndex % int(config.GridZ))
		rho := 0.05 / float64(carrierCount)

		if depositErr := solver.DepositCell(0, 0, cellZ, rho, 0, 0, 0, rho*config.CV); depositErr != nil {
			t.Fatal(depositErr)
		}
	}

	omega := 2 * math.Pi / config.DeltaT
	oscillators := make([]Oscillator, carrierCount)

	for index := range oscillators {
		perCarrierEnergy := config.RhoMin / float64(carrierCount)
		oscillators[index] = Oscillator{
			Phase:     float64(index) * 0.1,
			Omega:     omega,
			Amplitude: math.Sqrt(perCarrierEnergy),
			PosX:      1,
			PosY:      0,
			PosZ:      1,
			Heat:      perCarrierEnergy,
		}
	}

	if err := solver.SetOscillators(oscillators); err != nil {
		t.Fatal(err)
	}

	if _, err := solver.Step(); err != nil {
		t.Fatal(err)
	}

	readback, err := solver.ReadOscillators(carrierCount)

	if err != nil {
		t.Fatal(err)
	}

	if math.IsNaN(readback[0].Phase) {
		t.Fatalf("phase NaN with scaled multi-symbol deposits")
	}
}

func TestSpreadDepositOscCount(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		count  int
		spread bool
	}{
		{"1osc-1cell", 1, false},
		{"8osc-1cell", 8, false},
		{"128osc-spread-z", 128, true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			config := productionTestConfig()
			solver := NewSolver(config)

			defer solver.Close()

			solver.ResetDeposits()
			oscCount := testCase.count

			if testCase.name == "8osc-spread-1osc" {
				oscCount = 1
			}

			for index := 0; index < testCase.count; index++ {
				rho := 0.05 / float64(testCase.count)
				cellX := uint32(0)
				cellY := uint32(0)
				cellZ := uint32(0)

				if testCase.spread {
					if testCase.name == "128osc-spread-z" {
						cellZ = uint32(index % 16)
					} else {
						cellX = uint32(index % 32)
						cellZ = uint32(index % 16)
					}
				}

				solver.DepositCell(cellX, cellY, cellZ, rho, 0, 0, 0, rho*config.CV)
			}

			omega := 2 * math.Pi / config.DeltaT
			oscillators := make([]Oscillator, oscCount)

			for index := range oscillators {
				perCarrierEnergy := config.RhoMin / float64(oscCount)
				oscillators[index] = Oscillator{
					Phase: float64(index) * 0.1, Omega: omega,
					Amplitude: math.Sqrt(perCarrierEnergy), PosX: 1, PosY: 0, PosZ: 1,
					Heat: perCarrierEnergy,
				}
			}

			solver.SetOscillators(oscillators)
			solver.Step()
			readback, _ := solver.ReadOscillators(oscCount)

			if math.IsNaN(readback[0].Phase) {
				t.Fatalf("NaN")
			}
		})
	}
}

func TestSingleCarrierDepositMagnitude(t *testing.T) {
	config := productionTestConfig()
	solver := NewSolver(config)

	defer solver.Close()

	solver.ResetDeposits()

	for _, rho := range []float64{0.05, config.RhoMin / float64(config.MaxModes), config.RhoMin / 8} {
		t.Run(fmt.Sprintf("rho=%g", rho), func(t *testing.T) {
			solver.ResetDeposits()
			solver.DepositCell(1, 0, 1, rho, 0, 0, 0, rho*config.CV)
			omega := 2 * math.Pi / config.DeltaT
			perCarrierEnergy := config.RhoMin
			osc := []Oscillator{{
				Phase: 0, Omega: omega, Amplitude: math.Sqrt(perCarrierEnergy),
				PosX: 1, PosY: 0, PosZ: 1, Heat: perCarrierEnergy,
			}}
			solver.SetOscillators(osc)
			solver.Step()
			read, _ := solver.ReadOscillators(1)

			if math.IsNaN(read[0].Phase) {
				t.Fatalf("NaN at rho=%g", rho)
			}
		})
	}
}

func BenchmarkSolverStep(b *testing.B) {
	config := Config{
		GridX:    16,
		GridY:    1,
		GridZ:    16,
		DomainX:  0.32,
		DomainY:  1,
		DomainZ:  16,
		DeltaT:   0.1,
		Gamma:    5.0 / 3.0,
		MaxModes: 8,
	}
	ApplyDerivedGasParams(&config)

	solver := NewSolver(config)

	defer solver.Close()

	oscillators := make([]Oscillator, 8)

	for index := range oscillators {
		oscillators[index] = Oscillator{
			Phase:     float64(index) * 0.1,
			Omega:     6.28,
			Amplitude: 0.1,
			PosX:      float64(index),
			PosY:      0,
			PosZ:      float64(index),
			Heat:      0.1,
		}
	}

	if err := solver.SetOscillators(oscillators); err != nil {
		b.Fatal(err)
	}

	if err := solver.DepositCell(8, 0, 4, 1, 0.2, 0, 0, 0.5); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()

	for b.Loop() {
		if err := solver.ResetDeposits(); err != nil {
			b.Fatal(err)
		}

		if err := solver.DepositCell(8, 0, 4, 1, 0.2, 0, 0, 0.5); err != nil {
			b.Fatal(err)
		}

		if _, err := solver.Step(); err != nil {
			b.Fatal(err)
		}
	}
}

func TestMaxModesVsOscCount(t *testing.T) {
	for _, testCase := range []struct {
		maxModes int
		numOsc   int
	}{
		{32, 32},
		{128, 32},
		{128, 128},
	} {
		t.Run(fmt.Sprintf("max=%d-osc=%d", testCase.maxModes, testCase.numOsc), func(t *testing.T) {
			config := productionTestConfig()
			config.MaxModes = uint32(testCase.maxModes)

			solver := NewSolver(config)

			defer solver.Close()

			if err := solver.ResetDeposits(); err != nil {
				t.Fatal(err)
			}

			if err := solver.DepositCell(1, 0, 1, 0.05, 0, 0, 0, 0.05); err != nil {
				t.Fatal(err)
			}

			omega := 2 * math.Pi / config.DeltaT
			oscillators := make([]Oscillator, testCase.numOsc)

			for index := range oscillators {
				perCarrierEnergy := config.RhoMin / float64(testCase.numOsc)
				oscillators[index] = Oscillator{
					Phase:     float64(index) * 0.1,
					Omega:     omega,
					Amplitude: math.Sqrt(perCarrierEnergy),
					PosX:      1,
					PosY:      0,
					PosZ:      1,
					Heat:      perCarrierEnergy,
				}
			}

			if err := solver.SetOscillators(oscillators); err != nil {
				t.Fatal(err)
			}

			if _, err := solver.Step(); err != nil {
				t.Fatal(err)
			}

			readback, err := solver.ReadOscillators(testCase.numOsc)

			if err != nil {
				t.Fatal(err)
			}

			if math.IsNaN(readback[0].Phase) {
				t.Fatalf("phase NaN")
			}
		})
	}
}

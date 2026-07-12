//go:build darwin && cgo

package manifold

import "testing"

func trySteps(t *testing.T, config Config, numOsc uint32, steps int) {
	t.Helper()
	config = config.stableGasTestConfig(0, 1)
	solver := NewSolver(config)
	defer solver.Close()
	if err := solver.ResetDeposits(); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if err := solver.DepositCell(1, 0, 1, 0.05, 0, 0, 0, 0.05); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	osc := make([]Oscillator, numOsc)
	for i := range osc {
		posX, posY, posZ := config.testCellCenter(1, 0, 1)
		osc[i] = Oscillator{
			Omega:     6.28,
			Amplitude: 0.1,
			PosX:      posX,
			PosY:      posY,
			PosZ:      posZ,
			Heat:      0.1,
		}
	}
	if err := solver.SetOscillators(osc); err != nil {
		t.Fatalf("osc: %v", err)
	}
	for step := 0; step < steps; step++ {
		if _, err := solver.Step(); err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
	}
}

func tryStep(t *testing.T, config Config, numOsc uint32) {
	trySteps(t, config, numOsc, 1)
}

func TestBisectGridY3(t *testing.T) {
	tryStep(t, Config{GridX: 8, GridY: 3, GridZ: 8, DomainX: 0.16, DomainY: 3, DomainZ: 8, DeltaT: 0.1, Gamma: 5.0 / 3.0, MaxModes: 4}, 1)
}

func TestBisect32Osc(t *testing.T) {
	tryStep(t, Config{GridX: 8, GridY: 1, GridZ: 8, DomainX: 0.16, DomainY: 1, DomainZ: 8, DeltaT: 0.1, Gamma: 5.0 / 3.0, MaxModes: 32}, 32)
}

func TestBisectBigGrid(t *testing.T) {
	tryStep(t, Config{GridX: 32, GridY: 1, GridZ: 16, DomainX: 0.32, DomainY: 1, DomainZ: 16, DeltaT: 0.1, Gamma: 5.0 / 3.0, MaxModes: 32}, 32)
}

func TestBisectProductionLite(t *testing.T) {
	tryStep(t, Config{GridX: 32, GridY: 3, GridZ: 16, DomainX: 0.32, DomainY: 3, DomainZ: 16, DeltaT: 0.1, Gamma: 5.0 / 3.0, MaxModes: 32}, 32)
}

func TestBisectProduction128Osc(t *testing.T) {
	tryStep(t, Config{GridX: 32, GridY: 3, GridZ: 16, DomainX: 0.65, DomainY: 3, DomainZ: 16, DeltaT: 0.1, Gamma: 5.0 / 3.0, MaxModes: 128}, 128)
}

func TestBisectProduction128OscSustained(t *testing.T) {
	trySteps(t, Config{GridX: 32, GridY: 3, GridZ: 16, DomainX: 0.65, DomainY: 3, DomainZ: 16, DeltaT: 0.1, Gamma: 5.0 / 3.0, MaxModes: 128}, 128, 256)
}

//go:build darwin && cgo

package manifold

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestSolverStepStability(t *testing.T) {
	Convey("Given a production-scale coherence lattice", t, func() {
		config := productionTestConfig()
		config.BoundaryXLow = GasBoundaryPeriodic
		config.BoundaryXHigh = GasBoundaryPeriodic
		config.BoundaryYLow = GasBoundaryPeriodic
		config.BoundaryYHigh = GasBoundaryPeriodic
		config.BoundaryZLow = GasBoundaryPeriodic
		config.BoundaryZHigh = GasBoundaryPeriodic
		solver := NewSolver(config)

		defer solver.Close()

		oscillators := make([]Oscillator, config.MaxModes)

		for index := range oscillators {
			oscillators[index] = Oscillator{
				Phase:     float64(index) * 0.1,
				Omega:     6.28,
				Amplitude: 0.1,
				PosX:      1,
				PosY:      0,
				PosZ:      1,
				Heat:      0.1,
			}
		}

		So(solver.SetOscillators(oscillators), ShouldBeNil)
		So(solver.ResetDeposits(), ShouldBeNil)
		So(solver.DepositCell(1, 0, 1, 0.05, 0, 0, 0, 0.05), ShouldBeNil)

		Convey("It should keep coherence observables finite under sustained stepping", func() {
			for range 256 {
				reading, err := solver.Step()

				So(err, ShouldBeNil)
				So(reading.IsFinite(), ShouldBeTrue)
			}
		})
	})
}

func BenchmarkSolverStepProduction(b *testing.B) {
	config := productionTestConfig()
	solver := NewSolver(config)

	defer solver.Close()

	oscillators := make([]Oscillator, config.MaxModes)
	omega := 2 * math.Pi / config.DeltaT

	for index := range oscillators {
		oscillators[index] = Oscillator{
			Phase:     float64(index) * 0.1,
			Omega:     omega,
			Amplitude: 0.1,
			PosX:      1,
			PosY:      0,
			PosZ:      1,
			Heat:      0.1,
		}
	}

	if err := solver.SetOscillators(oscillators); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()

	for b.Loop() {
		if _, err := solver.Step(); err != nil {
			b.Fatal(err)
		}
	}
}

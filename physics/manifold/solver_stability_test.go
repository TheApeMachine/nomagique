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
		solver := NewSolver(config)

		defer solver.Close()

		oscillators := make([]Oscillator, config.MaxModes)
		energy := config.RhoMin / float64(len(oscillators))
		omega := 2 * math.Pi / config.DeltaT

		for index := range oscillators {
			oscillators[index] = Oscillator{
				Phase:     float64(index) * 0.1,
				Omega:     omega,
				Amplitude: math.Sqrt(energy),
				PosX:      1,
				PosY:      0,
				PosZ:      1,
				Heat:      energy,
			}
		}

		So(solver.SetOscillators(oscillators), ShouldBeNil)

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
	energy := config.RhoMin / float64(len(oscillators))
	omega := 2 * math.Pi / config.DeltaT

	for index := range oscillators {
		oscillators[index] = Oscillator{
			Phase:     float64(index) * 0.1,
			Omega:     omega,
			Amplitude: math.Sqrt(energy),
			PosX:      1,
			PosY:      0,
			PosZ:      1,
			Heat:      energy,
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

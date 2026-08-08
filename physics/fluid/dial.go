package fluid

import (
	"fmt"
	"math"
	"math/cmplx"

	"github.com/theapemachine/nomagique/geometry"
)

/*
SourceDial projects one contributor's particles onto the resident omega lattice
and reads the shared field through them.

Why:

	A Domain is a single gas for every contributor that appends to it, so Wave is a
	population-wide spectrum carrying no attribution of its own. The particles a
	contributor injected do carry it: each one holds the omega its content mapped to
	and the phase its sequence position assigned, so binning them onto the same
	lattice the wave occupies yields that contributor's occupancy spectrum.

	Multiplying occupancy by the resident mode is what makes the result a reading of
	the field rather than a restatement of the input. A contributor only lights up a
	mode it both occupies and the wave is actually excited at, which is the coupling
	the fingerprint is meant to expose.

	Attribution through spatial IDs is not equivalent: those retain only the low
	eight bits of the content token, which aliases distinct contributors together
	once their count passes the width of that field.
*/
func (domain *Domain) SourceDial(particles []Particle) (geometry.PhaseDial, error) {
	wave, err := domain.Wave()

	if err != nil {
		return nil, err
	}

	return projectDial(particles, wave, domain.config)
}

/*
projectDial is the pure projection, separated from the domain read so it can be
exercised against an explicit lattice.
*/
func projectDial(
	particles []Particle,
	wave []WaveMode,
	config Config,
) (geometry.PhaseDial, error) {
	if len(particles) == 0 || len(wave) == 0 {
		return nil, nil
	}

	span := float64(config.OmegaMax - config.OmegaMin)

	if !(span > 0) {
		return nil, fmt.Errorf("fluid: omega span must be positive")
	}

	occupancy := make([]complex128, len(wave))

	for _, particle := range particles {
		mass := float64(particle.Mass)

		if !(mass > 0) || math.IsInf(mass, 0) {
			continue
		}

		position := (float64(particle.Omega) - float64(config.OmegaMin)) / span
		bin := min(max(int(position*float64(len(wave))), 0), len(wave)-1)
		occupancy[bin] += cmplx.Rect(mass, float64(particle.Phase))
	}

	dial := make(geometry.PhaseDial, len(wave))

	var energy float64

	for index, mode := range wave {
		dial[index] = occupancy[index] * complex(
			float64(mode.Real), float64(mode.Imaginary),
		)
		magnitude := cmplx.Abs(dial[index])

		if math.IsNaN(magnitude) || math.IsInf(magnitude, 0) {
			return nil, fmt.Errorf("fluid: projected dial is not finite")
		}

		energy += magnitude * magnitude
	}

	if !(energy > 0) {
		return nil, nil
	}

	return dial, nil
}

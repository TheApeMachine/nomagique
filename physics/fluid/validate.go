//go:build darwin && cgo

package fluid

import (
	"fmt"
	"math"
)

/*
validateParticles rejects an empty or physically inadmissible population before
it reaches shared Metal buffers.
*/
func validateParticles(particles []Particle, config Config) error {
	if len(particles) == 0 {
		return fmt.Errorf("fluid: particle state is empty")
	}

	for index, particle := range particles {
		values := []float32{
			particle.Position.X,
			particle.Position.Y,
			particle.Position.Z,
			particle.Velocity.X,
			particle.Velocity.Y,
			particle.Velocity.Z,
			particle.Mass,
			particle.Heat,
			particle.Energy,
			particle.Phase,
			particle.Omega,
		}

		for _, value := range values {
			if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
				return fmt.Errorf("fluid: particle %d contains a non-finite value", index)
			}
		}

		if particle.Mass <= MinimumPilotWaveMass || particle.Heat < 0 || particle.Energy < 0 {
			return fmt.Errorf(
				"fluid: particle %d has inadmissible mass or energy - mass %g, heat %g, energy %g",
				index,
				particle.Mass,
				particle.Heat,
				particle.Energy,
			)
		}

		if particle.Omega < config.OmegaMin || particle.Omega > config.OmegaMax {
			return fmt.Errorf(
				"fluid: particle %d omega %g is outside [%g,%g]",
				index,
				particle.Omega,
				config.OmegaMin,
				config.OmegaMax,
			)
		}
	}

	return nil
}

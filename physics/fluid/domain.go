package fluid

import (
	"fmt"
	"math"
)

const maximumWaveModes = 128

/*
Config defines the public controls exposed by the Sensorium Metal domain: the
periodic grid topology, dynamically derived timestep ceiling, and explicit
omega lattice bounds shared by every input batch.
*/
type Config struct {
	Grid     Grid
	MaxDelta float32
	OmegaMin float32
	OmegaMax float32
}

/*
DefaultConfig returns the topology, timestep ceiling, and [-4,4] omega interval
of the original Sensorium implementation.
*/
func DefaultConfig() Config {
	return Config{
		Grid: Grid{
			X:       64,
			Y:       64,
			Z:       64,
			Spacing: 1.0 / 64.0,
		},
		MaxDelta: 0.015,
		OmegaMin: -4,
		OmegaMax: 4,
	}
}

/*
Validate rejects a domain whose explicit public controls cannot support the
fixed Sensorium integration model.
*/
func (config Config) Validate() error {
	if err := config.Grid.Validate(); err != nil {
		return err
	}

	if config.MaxDelta <= 0 || math.IsNaN(float64(config.MaxDelta)) ||
		math.IsInf(float64(config.MaxDelta), 0) {
		return fmt.Errorf("fluid: maximum timestep must be finite and positive")
	}

	if !finite(config.OmegaMin) || !finite(config.OmegaMax) ||
		config.OmegaMax <= config.OmegaMin {
		return fmt.Errorf("fluid: omega bounds must be finite and increasing")
	}

	derivedSpacing := float32(1.0 / float64(max(config.Grid.X, config.Grid.Y, config.Grid.Z)))

	if config.Grid.Spacing != derivedSpacing {
		return fmt.Errorf(
			"fluid: grid spacing must equal the normalized topology spacing %g",
			derivedSpacing,
		)
	}

	waveModes := 1

	for waveModes < max(config.Grid.X, config.Grid.Y, config.Grid.Z) {
		waveModes <<= 1
	}

	if waveModes > maximumWaveModes {
		return fmt.Errorf(
			"fluid: derived omega lattice has %d modes; Metal supports at most %d",
			waveModes,
			maximumWaveModes,
		)
	}

	return nil
}

/*
Particle is the complete Lagrangian state exchanged between the thermodynamic
and omega-wave domains. Mass is inertial mass in the pilot-wave equation, not
a population-normalized probability weight. Energy is oscillator energy; Heat
is thermal energy.
*/
type Particle struct {
	Position Vector
	Velocity Vector
	Mass     float32
	Heat     float32
	Energy   float32
	Phase    float32
	Omega    float32
}

/*
Diagnostics records gas stability, omega-wave amplitude evolution, and spatial
pilot-wave guidance from the most recent coupled step.
*/
type Diagnostics struct {
	CFLRate      float32
	DeltaAdv     float32
	DeltaDiffuse float32
	DeltaDerived float32
	DeltaUsed    float32
	Halvings     uint32
	PsiRMS       float32
	PsiDeltaRMS  float32
	GuidanceRMS  float32
}

/*
WaveMode is one site of the uniform omega lattice after the dissipative GPE
update.
*/
type WaveMode struct {
	Omega     float32 `json:"omega"`
	Real      float32 `json:"real"`
	Imaginary float32 `json:"imaginary"`
	Linewidth float32 `json:"linewidth"`
}

/*
Reading is the resident field observation consumed after a coupled step. Gas
derivatives are evaluated at the periodic particle centroid; coherence and
guidance summarize the complete resident wave and particle populations.
*/
type Reading struct {
	PressureGradX    float64 `json:"pressureGradX"`
	PressureGradY    float64 `json:"pressureGradY"`
	PressureGradZ    float64 `json:"pressureGradZ"`
	PressureGradNorm float64 `json:"pressureGradNorm"`
	Divergence       float64 `json:"divergence"`
	CoherenceMag2    float64 `json:"coherenceMag2"`
	GuidanceSpeed    float64 `json:"guidanceSpeed"`
	ViscosityProxy   float64 `json:"viscosityProxy"`
}

/*
IsFinite reports whether every physical field observation is finite.
*/
func (reading Reading) IsFinite() bool {
	values := []float64{
		reading.PressureGradX,
		reading.PressureGradY,
		reading.PressureGradZ,
		reading.PressureGradNorm,
		reading.Divergence,
		reading.CoherenceMag2,
		reading.GuidanceSpeed,
		reading.ViscosityProxy,
	}

	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}

	return true
}

/*
finite reports whether one binary32 domain control is a real number.
*/
func finite(value float32) bool {
	return !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0)
}

/*
Projection is the X-Z maximum projection of the resident gas and spatial wave
fields. Every slice is row-major with X varying fastest and has Grid.X*Grid.Z
elements.
*/
type Projection struct {
	Grid      Grid
	Density   []float32
	Coherence []float32
	GuidanceX []float32
	GuidanceZ []float32
}

/*
Domain owns the resident Metal fields for the coupled thermodynamic and
omega-wave implementation. Platform files provide the Metal handle.
*/
type Domain struct {
	handle domainHandle
	config Config
}

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

		if particle.Mass <= 0 || particle.Heat < 0 || particle.Energy < 0 {
			return fmt.Errorf("fluid: particle %d has inadmissible mass or energy", index)
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

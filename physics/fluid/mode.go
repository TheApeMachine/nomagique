package fluid

import (
	"fmt"
	"math"
)

/*
ModeState identifies a spectral mode by its relation to the thermal noise floor
and its phase coherence.
*/
type ModeState uint8

const (
	ModeThermal ModeState = iota
	ModeExcited
	ModeCoherent
	ModeCondensed
)

/*
LindbladDecayRate derives the thermal relaxation rate for a dimensionless bath
coupling.
*/
func (constants PhysicalConstants) LindbladDecayRate(
	temperature, coupling float64,
) float64 {
	return coupling * constants.ThermalFrequency(temperature)
}

/*
ModePhysics contains the thermal scales and timestep-dependent dynamics needed
by coherence modes. Every field is derived from its declared physical inputs.
*/
type ModePhysics struct {
	Temperature      float64
	Delta            float64
	ThermalFrequency float64
	CoherenceTime    float64
	ThermalAmplitude float64
	BandwidthMinimum float64
	BandwidthMaximum float64
	DecayRate        float64
	DecayFactor      float64
	CoherenceSteps   int
}

/*
NewModePhysics derives one mode-physics snapshot. Bath coupling and effective
mass remain explicit physical inputs; invalid values return errors instead of
being silently replaced.
*/
func NewModePhysics(
	temperature, delta float64,
	constants PhysicalConstants,
	bathCoupling, effectiveMass float64,
) (ModePhysics, error) {
	if temperature <= 0 || !finitePositive(delta) {
		return ModePhysics{}, fmt.Errorf("fluid: mode temperature and timestep must be positive")
	}

	if !finitePositive(effectiveMass) || !finitePositive(bathCoupling) {
		return ModePhysics{}, fmt.Errorf("fluid: mode bath coupling and effective mass must be positive")
	}

	if err := constants.Validate(); err != nil {
		return ModePhysics{}, err
	}

	thermalFrequency := constants.ThermalFrequency(temperature)
	coherenceTime := constants.ThermalCoherenceTime(temperature)
	decayRate := constants.LindbladDecayRate(temperature, bathCoupling)
	decayFactor := 0.0

	if decayRate*delta < 700 {
		decayFactor = math.Exp(-decayRate * delta)
	}

	return ModePhysics{
		Temperature:      temperature,
		Delta:            delta,
		ThermalFrequency: thermalFrequency,
		CoherenceTime:    coherenceTime,
		ThermalAmplitude: math.Sqrt(constants.KB * temperature / effectiveMass),
		BandwidthMinimum: UncertaintyBandwidth(delta),
		BandwidthMaximum: thermalFrequency,
		DecayRate:        decayRate,
		DecayFactor:      decayFactor,
		CoherenceSteps:   max(1, int(coherenceTime/delta)),
	}, nil
}

/*
NoiseFloor returns the zero-point plus thermal amplitude at one frequency.
*/
func (physics ModePhysics) NoiseFloor(
	omega float64,
	constants PhysicalConstants,
	effectiveMass float64,
) float64 {
	return constants.ThermalAmplitude(omega, physics.Temperature, effectiveMass)
}

/*
Visible reports whether a mode rises above its derived thermal noise floor.
*/
func (physics ModePhysics) Visible(
	amplitude, omega float64,
	constants PhysicalConstants,
	effectiveMass float64,
) bool {
	return amplitude > physics.NoiseFloor(omega, constants, effectiveMass)
}

/*
VisibilityRatio returns the mode amplitude divided by its thermal floor.
*/
func (physics ModePhysics) VisibilityRatio(
	amplitude, omega float64,
	constants PhysicalConstants,
	effectiveMass float64,
) float64 {
	return constants.ModeVisibilityRatio(
		amplitude,
		omega,
		physics.Temperature,
		effectiveMass,
	)
}

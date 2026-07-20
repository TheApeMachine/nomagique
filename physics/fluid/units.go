package fluid

import (
	"fmt"
	"math"
)

/*
SIConstants contains the defined and CODATA constants consumed by the fluid
model. Keeping their SI provenance separate from simulation units prevents a
unit conversion from becoming an undocumented model parameter.
*/
type SIConstants struct {
	G       float64
	KB      float64
	NA      float64
	SigmaSB float64
	H       float64
}

/*
DefaultSIConstants returns the constants used by the Sensorium implementation.
The values are physical definitions or CODATA measurements rather than tuning
parameters.
*/
func DefaultSIConstants() SIConstants {
	return SIConstants{
		G:       6.67430e-11,
		KB:      1.380649e-23,
		NA:      6.02214076e23,
		SigmaSB: 5.670374419e-8,
		H:       6.62607015e-34,
	}
}

/*
UnitSystem declares how one simulation base unit maps to SI. It is a statement
of dimensional meaning, not an optimization or calibration surface.
*/
type UnitSystem struct {
	LengthMetres      float64
	MassKilograms     float64
	TimeSeconds       float64
	TemperatureKelvin float64
	Name              string
}

/*
SIUnitSystem returns the identity mapping in which simulation and SI base units
are equal.
*/
func SIUnitSystem(name string) UnitSystem {
	return UnitSystem{
		LengthMetres:      1,
		MassKilograms:     1,
		TimeSeconds:       1,
		TemperatureKelvin: 1,
		Name:              name,
	}
}

/*
OmegaNaturalUnitSystem derives the Sensorium natural units in which hbar, k_B,
and c_v are one. This keeps the oscillator crossover omega approximately equal
to temperature without introducing an independently tuned scale.
*/
func OmegaNaturalUnitSystem(
	gamma, molecularWeight, lengthMetres float64,
	constants SIConstants,
) (UnitSystem, error) {
	if gamma <= 1 {
		return UnitSystem{}, fmt.Errorf("fluid: gamma must be greater than one")
	}

	if molecularWeight <= 0 || lengthMetres <= 0 {
		return UnitSystem{}, fmt.Errorf("fluid: molecular weight and length unit must be positive")
	}

	gasConstant := constants.NA * constants.KB / molecularWeight
	hbar := constants.H / (2 * math.Pi)

	if !finitePositive(gasConstant) || !finitePositive(hbar) {
		return UnitSystem{}, fmt.Errorf("fluid: natural-unit inputs are not finite and positive")
	}

	massKilograms := (gamma - 1) * constants.KB / gasConstant
	timeSeconds := lengthMetres * lengthMetres * massKilograms / hbar
	temperatureKelvin := hbar * hbar /
		(constants.KB * lengthMetres * lengthMetres * massKilograms)

	return UnitSystem{
		LengthMetres:      lengthMetres,
		MassKilograms:     massKilograms,
		TimeSeconds:       timeSeconds,
		TemperatureKelvin: temperatureKelvin,
		Name:              "omega_natural",
	}, nil
}

/*
PhysicalConstants contains universal constants expressed in one UnitSystem.
Source records their provenance for diagnostics and serialized experiments.
*/
type PhysicalConstants struct {
	Source  string
	G       float64
	KB      float64
	SigmaSB float64
	HBar    float64
}

/*
ConstantsFromSI converts the CODATA constants into the supplied simulation unit
system by applying their base-unit exponents directly.
*/
func ConstantsFromSI(units UnitSystem, constants SIConstants) (PhysicalConstants, error) {
	length := units.LengthMetres
	mass := units.MassKilograms
	time := units.TimeSeconds
	temperature := units.TemperatureKelvin

	if !finitePositive(length) || !finitePositive(mass) ||
		!finitePositive(time) || !finitePositive(temperature) {
		return PhysicalConstants{}, fmt.Errorf("fluid: unit-system bases must be finite and positive")
	}

	converted := PhysicalConstants{
		Source: "codata_si_derived",
		G: convertSI(
			constants.G, length, mass, time, temperature,
			[4]int{3, -1, -2, 0},
		),
		KB: convertSI(
			constants.KB, length, mass, time, temperature,
			[4]int{2, 1, -2, -1},
		),
		SigmaSB: convertSI(
			constants.SigmaSB, length, mass, time, temperature,
			[4]int{0, 1, -3, -4},
		),
		HBar: convertSI(
			constants.H/(2*math.Pi), length, mass, time, temperature,
			[4]int{2, 1, -1, 0},
		),
	}

	if err := converted.Validate(); err != nil {
		return PhysicalConstants{}, err
	}

	return converted, nil
}

/*
Validate rejects non-finite physical constants before they can contaminate a
field update.
*/
func (constants PhysicalConstants) Validate() error {
	values := map[string]float64{
		"G":        constants.G,
		"k_B":      constants.KB,
		"sigma_SB": constants.SigmaSB,
		"hbar":     constants.HBar,
	}

	for name, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("fluid: non-finite physical constant %s=%g", name, value)
		}
	}

	return nil
}

/*
GasSpecificConstant returns the mass-specific ideal-gas constant in simulation
units for a medium identified by its SI molar mass.
*/
func (units UnitSystem) GasSpecificConstant(
	molecularWeight float64, constants SIConstants,
) (float64, error) {
	if molecularWeight <= 0 {
		return 0, fmt.Errorf("fluid: molecular weight must be positive")
	}

	gasConstantSI := constants.NA * constants.KB / molecularWeight
	value := convertSI(
		gasConstantSI,
		units.LengthMetres,
		units.MassKilograms,
		units.TimeSeconds,
		units.TemperatureKelvin,
		[4]int{2, 0, -2, -1},
	)

	if !finitePositive(value) {
		return 0, fmt.Errorf("fluid: specific gas constant is not finite and positive")
	}

	return value, nil
}

/*
DynamicViscosity converts SI Pa-s into the declared simulation units. A
non-positive or non-finite material value denotes the absence of viscosity, as
in the Sensorium reference.
*/
func (units UnitSystem) DynamicViscosity(valueSI float64) float64 {
	if !finitePositive(valueSI) {
		return 0
	}

	return convertSI(
		valueSI,
		units.LengthMetres,
		units.MassKilograms,
		units.TimeSeconds,
		units.TemperatureKelvin,
		[4]int{-1, 1, -1, 0},
	)
}

/*
BoseEinsteinOccupation returns the mean occupation of a harmonic oscillator in
a thermal bath.
*/
func (constants PhysicalConstants) BoseEinsteinOccupation(omega, temperature float64) float64 {
	if temperature <= 0 || omega <= 0 {
		return 0
	}

	exponent := constants.HBar * omega / (constants.KB * temperature)

	if exponent > 700 {
		return 0
	}

	return 1 / math.Expm1(exponent)
}

/*
ThermalAmplitude returns the RMS zero-point plus thermal fluctuation amplitude
of a harmonic oscillator.
*/
func (constants PhysicalConstants) ThermalAmplitude(
	omega, temperature, effectiveMass float64,
) float64 {
	if omega <= 0 || effectiveMass <= 0 {
		return 0
	}

	occupation := constants.BoseEinsteinOccupation(omega, temperature)
	return math.Sqrt(constants.HBar / (2 * effectiveMass * omega) * (2*occupation + 1))
}

/*
ThermalCoherenceTime returns hbar/(k_B T), or positive infinity at absolute
zero where this thermal decoherence channel is absent.
*/
func (constants PhysicalConstants) ThermalCoherenceTime(temperature float64) float64 {
	if temperature <= 0 {
		return math.Inf(1)
	}

	return constants.HBar / (constants.KB * temperature)
}

/*
ThermalFrequency returns the inverse thermal coherence timescale k_B T/hbar.
*/
func (constants PhysicalConstants) ThermalFrequency(temperature float64) float64 {
	if temperature <= 0 {
		return 0
	}

	return constants.KB * temperature / constants.HBar
}

/*
UncertaintyBandwidth returns the minimum resolvable angular-frequency width for
an observation lasting deltaTime.
*/
func UncertaintyBandwidth(deltaTime float64) float64 {
	if deltaTime <= 0 {
		return math.Inf(1)
	}

	return 0.5 / deltaTime
}

/*
ModeVisibilityRatio measures a mode amplitude against its thermal fluctuation
floor.
*/
func (constants PhysicalConstants) ModeVisibilityRatio(
	amplitude, omega, temperature, effectiveMass float64,
) float64 {
	thermal := constants.ThermalAmplitude(omega, temperature, effectiveMass)

	if thermal > 0 {
		return amplitude / thermal
	}

	if amplitude > 0 {
		return math.Inf(1)
	}

	return 0
}

/*
QuantumThermodynamicState caches the bath quantities shared by all spectral
modes during one step.
*/
type QuantumThermodynamicState struct {
	Temperature   float64
	Frequency     float64
	CoherenceTime float64
}

/*
QuantumState derives the shared spectral bath state from one temperature.
*/
func (constants PhysicalConstants) QuantumState(temperature float64) QuantumThermodynamicState {
	return QuantumThermodynamicState{
		Temperature:   temperature,
		Frequency:     constants.ThermalFrequency(temperature),
		CoherenceTime: constants.ThermalCoherenceTime(temperature),
	}
}

func convertSI(
	value, length, mass, time, temperature float64,
	dimensions [4]int,
) float64 {
	return value *
		math.Pow(length, float64(-dimensions[0])) *
		math.Pow(mass, float64(-dimensions[1])) *
		math.Pow(time, float64(-dimensions[2])) *
		math.Pow(temperature, float64(-dimensions[3]))
}

func finitePositive(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

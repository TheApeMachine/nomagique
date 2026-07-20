package fluid

import (
	"fmt"
	"math"
)

/*
GasNumerics declares the stability limits and admissibility floors of the
correctness-first periodic gas reference. These values are explicit numerical
choices from gas_dynamics.py rather than hidden recovery behavior.
*/
type GasNumerics struct {
	CFL             float32
	CFLDiffusion    float32
	DensityMinimum  float32
	PressureMinimum float32
}

/*
DefaultGasNumerics returns the documented Sensorium Rusanov/RK2 choices.
*/
func DefaultGasNumerics() GasNumerics {
	return GasNumerics{
		CFL:             0.4,
		CFLDiffusion:    0.15,
		DensityMinimum:  1e-3,
		PressureMinimum: 1e-3,
	}
}

/*
GasMaterial supplies the ideal-gas and constant-coefficient transport
properties required by the CPU reference solver.
*/
type GasMaterial struct {
	Gamma               float32
	SpecificGasConstant float32
	DynamicViscosity    float32
	ThermalConductivity float32
	SpecificHeat        float32
}

/*
GasState stores total-energy conserved variables on one periodic Cartesian
grid. This is the CPU-reference convention from gas_dynamics.py; the Metal
domain deliberately stores internal energy instead.
*/
type GasState struct {
	Density  []float32
	Momentum []Vector
	Energy   []float32
}

/*
NewGasState allocates a zeroed total-energy state for one grid.
*/
func NewGasState(grid Grid) (*GasState, error) {
	if err := grid.Validate(); err != nil {
		return nil, err
	}

	return &GasState{
		Density:  make([]float32, grid.Cells()),
		Momentum: make([]Vector, grid.Cells()),
		Energy:   make([]float32, grid.Cells()),
	}, nil
}

/*
Gas owns the parameters of the standalone Torch-equivalent reference solver.
It is intentionally independent of Domain so it cannot become a silent Metal
fallback.
*/
type Gas struct {
	grid     Grid
	material GasMaterial
	numerics GasNumerics
	gravity  []Vector
}

/*
NewGas validates and constructs the periodic CPU gas reference. Gravity may be
empty or contain one acceleration vector per cell.
*/
func NewGas(
	grid Grid,
	material GasMaterial,
	numerics GasNumerics,
	gravity []Vector,
) (*Gas, error) {
	gas := &Gas{
		grid:     grid,
		material: material,
		numerics: numerics,
		gravity:  gravity,
	}

	if err := gas.Validate(); err != nil {
		return nil, err
	}

	return gas, nil
}

/*
Validate rejects incomplete physical or numerical declarations before a field
is advanced.
*/
func (gas *Gas) Validate() error {
	if gas == nil {
		return fmt.Errorf("fluid: gas model is nil")
	}

	if err := gas.grid.Validate(); err != nil {
		return err
	}

	values := map[string]float32{
		"gamma":                 gas.material.Gamma,
		"specific gas constant": gas.material.SpecificGasConstant,
		"specific heat":         gas.material.SpecificHeat,
		"CFL":                   gas.numerics.CFL,
		"diffusion CFL":         gas.numerics.CFLDiffusion,
		"density minimum":       gas.numerics.DensityMinimum,
		"pressure minimum":      gas.numerics.PressureMinimum,
	}

	for name, value := range values {
		if !finitePositive32(value) {
			return fmt.Errorf("fluid: gas %s must be finite and positive", name)
		}
	}

	if gas.material.Gamma <= 1 {
		return fmt.Errorf("fluid: gas gamma must be greater than one")
	}

	if !finiteNonNegative32(gas.material.DynamicViscosity) ||
		!finiteNonNegative32(gas.material.ThermalConductivity) {
		return fmt.Errorf("fluid: gas transport coefficients must be finite and non-negative")
	}

	if len(gas.gravity) != 0 && len(gas.gravity) != gas.grid.Cells() {
		return fmt.Errorf("fluid: gravity field does not match the gas grid")
	}

	return nil
}

/*
StableDelta computes the unsplit multi-dimensional advective CFL bound and the
explicit viscous and conductive bounds used by gas_dynamics.py.
*/
func (gas *Gas) StableDelta(state *GasState) (float32, error) {
	if err := gas.validateState(state); err != nil {
		return 0, err
	}

	var maximumRate float32

	for cell := range state.Density {
		density, velocity, pressure := gas.primitive(state, cell)
		sound := float32(math.Sqrt(float64(gas.material.Gamma * pressure / density)))
		rate := (abs32(velocity.X) + abs32(velocity.Y) +
			abs32(velocity.Z) + 3*sound) / gas.grid.Spacing
		maximumRate = max(maximumRate, rate)
	}

	delta := float32(math.Inf(1))

	if maximumRate > 0 {
		delta = gas.numerics.CFL / maximumRate
	}

	spacingSquared := gas.grid.Spacing * gas.grid.Spacing

	if gas.material.DynamicViscosity > 0 {
		viscosity := gas.material.DynamicViscosity / gas.numerics.DensityMinimum
		delta = min(delta, gas.numerics.CFLDiffusion*spacingSquared/viscosity)
	}

	if gas.material.ThermalConductivity > 0 {
		diffusivity := gas.material.ThermalConductivity /
			(gas.numerics.DensityMinimum * gas.material.SpecificHeat)
		delta = min(delta, gas.numerics.CFLDiffusion*spacingSquared/diffusivity)
	}

	return delta, nil
}

/*
Advance performs one positivity-projected Heun/RK2 step using Rusanov fluxes,
constant-coefficient viscosity and conduction, and the optional gravity field.
*/
func (gas *Gas) Advance(state *GasState, delta float32) (*GasState, error) {
	if err := gas.validateState(state); err != nil {
		return nil, err
	}

	if !finitePositive32(delta) {
		return nil, fmt.Errorf("fluid: gas timestep must be finite and positive")
	}

	first := gas.rhs(state)
	stage := gas.project(state.add(first, delta))
	second := gas.rhs(stage)
	next := gas.project(state.addAverage(first, second, delta))

	if err := gas.validateState(next); err != nil {
		return nil, err
	}

	return next, nil
}

/*
validateState confirms that one conserved state matches this gas topology and
contains only finite values.
*/
func (gas *Gas) validateState(state *GasState) error {
	if state == nil || len(state.Density) != gas.grid.Cells() ||
		len(state.Momentum) != gas.grid.Cells() || len(state.Energy) != gas.grid.Cells() {
		return fmt.Errorf("fluid: conserved gas state does not match the grid")
	}

	for cell := range state.Density {
		values := []float32{
			state.Density[cell], state.Momentum[cell].X, state.Momentum[cell].Y,
			state.Momentum[cell].Z, state.Energy[cell],
		}

		for _, value := range values {
			if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
				return fmt.Errorf("fluid: conserved gas state is not finite at cell %d", cell)
			}
		}
	}

	return nil
}

/*
clone copies a conserved state so RK stages never alias their input.
*/
func (state *GasState) clone() *GasState {
	return &GasState{
		Density:  append([]float32(nil), state.Density...),
		Momentum: append([]Vector(nil), state.Momentum...),
		Energy:   append([]float32(nil), state.Energy...),
	}
}

/*
add applies one scaled derivative to a copied conserved state.
*/
func (state *GasState) add(rate *GasState, scale float32) *GasState {
	next := state.clone()

	for cell := range next.Density {
		next.Density[cell] += scale * rate.Density[cell]
		next.Momentum[cell].X += scale * rate.Momentum[cell].X
		next.Momentum[cell].Y += scale * rate.Momentum[cell].Y
		next.Momentum[cell].Z += scale * rate.Momentum[cell].Z
		next.Energy[cell] += scale * rate.Energy[cell]
	}

	return next
}

/*
addAverage applies the Heun average of two derivatives to a copied state.
*/
func (state *GasState) addAverage(first, second *GasState, delta float32) *GasState {
	next := state.clone()
	scale := 0.5 * delta

	for cell := range next.Density {
		next.Density[cell] += scale * (first.Density[cell] + second.Density[cell])
		next.Momentum[cell].X += scale * (first.Momentum[cell].X + second.Momentum[cell].X)
		next.Momentum[cell].Y += scale * (first.Momentum[cell].Y + second.Momentum[cell].Y)
		next.Momentum[cell].Z += scale * (first.Momentum[cell].Z + second.Momentum[cell].Z)
		next.Energy[cell] += scale * (first.Energy[cell] + second.Energy[cell])
	}

	return next
}

func finitePositive32(value float32) bool {
	return value > 0 && !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0)
}

func finiteNonNegative32(value float32) bool {
	return value >= 0 && !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0)
}

func abs32(value float32) float32 {
	return float32(math.Abs(float64(value)))
}

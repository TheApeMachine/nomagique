package equation

import (
	"math"

	"github.com/theapemachine/errnie"
)

/*
Fluidflow measures laminar, turbulent, inertial, and viscous book-flow evidence
without selecting a market category.
*/
type Fluidflow struct{}

/*
FluidflowInput contains the float-only fluid-flow inputs.
*/
type FluidflowInput struct {
	Reynolds           float64
	Divergence         float64
	Viscosity          float64
	LaminarCeiling     float64
	TurbulentFloor     float64
	DivergenceEdge     float64
	ViscosityBaseline  float64
	Vorticity          float64
	VorticityBaseline  float64
	Turbulence         float64
	TurbulenceBaseline float64
}

/*
FluidflowOutput contains the float-only fluid-flow scores.
*/
type FluidflowOutput struct {
	LaminarScore   float64
	TurbulentScore float64
	InertialScore  float64
	ViscousScore   float64
}

/*
NewFluidflow returns a fluid-dynamics calculator.
*/
func NewFluidflow() *Fluidflow {
	return &Fluidflow{}
}

/*
Measure calculates fluid-flow scores from floats without artifact transport.
*/
func (fluidflow *Fluidflow) Measure(
	input FluidflowInput,
) (FluidflowOutput, error) {
	values := []float64{
		input.Reynolds,
		input.Divergence,
		input.Viscosity,
		input.LaminarCeiling,
		input.TurbulentFloor,
		input.DivergenceEdge,
		input.ViscosityBaseline,
		input.Vorticity,
		input.VorticityBaseline,
		input.Turbulence,
		input.TurbulenceBaseline,
	}

	for _, value := range values {
		if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return FluidflowOutput{}, errnie.Error(errnie.Err(
				errnie.Validation,
				"fluidflow: invalid mechanics input",
				nil,
			))
		}
	}

	reynoldsActivity := 0.0
	reynoldsTurbulence := 0.0
	divergenceActivity := 0.0
	viscosityActivity := 0.0
	vorticityActivity := 0.0
	turbulenceActivity := 0.0

	if input.LaminarCeiling > 0 {
		reynoldsActivity = input.Reynolds / input.LaminarCeiling
	}

	if input.TurbulentFloor > 0 {
		reynoldsTurbulence = input.Reynolds / input.TurbulentFloor
	}

	if input.DivergenceEdge > 0 {
		divergenceActivity = input.Divergence / input.DivergenceEdge
	}

	if input.ViscosityBaseline > 0 {
		viscosityActivity = input.Viscosity / input.ViscosityBaseline
	}

	if input.VorticityBaseline > 0 {
		vorticityActivity = input.Vorticity / input.VorticityBaseline
	}

	if input.TurbulenceBaseline > 0 {
		turbulenceActivity = input.Turbulence / input.TurbulenceBaseline
	}

	fieldActivity := math.Max(
		reynoldsActivity,
		math.Max(divergenceActivity, math.Max(vorticityActivity, turbulenceActivity)),
	)
	laminarScore := viscosityActivity / math.Max(1, fieldActivity)
	turbulentScore := math.Max(
		0,
		math.Max(
			reynoldsTurbulence,
			math.Max(vorticityActivity, turbulenceActivity),
		)-1,
	)
	inertialScore := math.Max(
		0,
		math.Min(reynoldsActivity, divergenceActivity)-1,
	)
	viscousScore := 0.0

	if input.Viscosity > 0 && input.ViscosityBaseline > 0 {
		viscousScore = math.Max(
			0,
			math.Min(input.ViscosityBaseline/input.Viscosity, divergenceActivity)-1,
		)
	}

	return FluidflowOutput{
		LaminarScore:   laminarScore,
		TurbulentScore: turbulentScore,
		InertialScore:  inertialScore,
		ViscousScore:   viscousScore,
	}, nil
}

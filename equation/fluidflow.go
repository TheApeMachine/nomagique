package equation

import (
	"math"

	"github.com/theapemachine/errnie"
)

/*
Fluidflow classifies laminar, turbulent, inertial, and viscous book-flow regimes.
*/
type Fluidflow struct{}

/*
FluidflowInput contains the float-only fluid-flow inputs.
*/
type FluidflowInput struct {
	Reynolds       float64
	Divergence     float64
	Viscosity      float64
	MidAddRate     float64
	MidExecuteRate float64
	LaminarCeiling float64
	TurbulentFloor float64
	TurbulentReady bool
	DivergenceEdge float64
	IcebergScore   float64
	Vorticity      float64
	Turbulence     float64
	Memory         float64
	Price          float64
	SpreadBPS      float64
	ChangePct      float64
	Volume         float64
}

/*
FluidflowOutput contains the float-only fluid-flow scores.
*/
type FluidflowOutput struct {
	Value          float64
	LaminarScore   float64
	TurbulentScore float64
	InertialScore  float64
	ViscousScore   float64
	Strength       float64
	Category       float64
	Price          float64
	SpreadBPS      float64
	ChangePct      float64
	Volume         float64
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
	if input.Price <= 0 ||
		input.SpreadBPS <= 0 ||
		input.Volume <= 0 ||
		math.IsNaN(input.ChangePct) ||
		math.IsInf(input.ChangePct, 0) {
		return FluidflowOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"fluidflow: invalid market input",
			nil,
		))
	}

	if input.Viscosity <= 0 ||
		math.IsNaN(input.Reynolds) ||
		math.IsInf(input.Reynolds, 0) {
		return FluidflowOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"fluidflow: invalid mechanics input",
			nil,
		))
	}

	divergenceEdge := input.DivergenceEdge
	if divergenceEdge <= 0 && input.Divergence > 0 {
		divergenceEdge = input.Divergence
	}

	laminarScore := 0.0
	lowReynolds := input.Reynolds <= 0 ||
		(input.LaminarCeiling > 0 && input.Reynolds <= input.LaminarCeiling)
	lowDivergence := input.Divergence <= 0 ||
		(divergenceEdge > 0 && input.Divergence <= divergenceEdge)

	if lowReynolds && lowDivergence {
		laminarScore = input.Viscosity

		if divergenceEdge > 0 && input.Divergence > 0 {
			laminarScore = input.Viscosity * (1 - input.Divergence/divergenceEdge)
		}
	}

	turbulentScore := 0.0

	if input.TurbulentReady && input.Reynolds >= input.TurbulentFloor {
		turbulentScore = input.Reynolds / input.TurbulentFloor
	}

	if input.Turbulence > 0 && input.TurbulentReady {
		turbulentFromField := input.Turbulence * input.Reynolds

		if turbulentFromField > turbulentScore {
			turbulentScore = turbulentFromField
		}
	}

	if input.Vorticity > 0 && input.TurbulentReady {
		vortScore := input.Vorticity * input.Turbulence

		if vortScore > turbulentScore {
			turbulentScore = vortScore
		}
	}

	inertialScore := input.Divergence
	viscousScore := 0.0

	if input.Viscosity > 0 {
		viscousScore = input.Divergence / input.Viscosity
	}

	if input.IcebergScore > 0 {
		viscousScore = math.Max(viscousScore, input.IcebergScore)
	}

	if !math.IsNaN(input.Memory) && !math.IsInf(input.Memory, 0) && input.Memory > 0 {
		viscousScore = math.Max(viscousScore, input.Memory*input.Viscosity)
	}

	midActivity := input.MidAddRate + input.MidExecuteRate

	if midActivity > 0 {
		executeShare := input.MidExecuteRate / midActivity
		addShare := input.MidAddRate / midActivity

		inertialScore = math.Max(inertialScore, executeShare*input.Divergence)

		if addShare > executeShare {
			laminarRegime := input.LaminarCeiling > 0 &&
				input.Reynolds <= input.LaminarCeiling &&
				divergenceEdge > 0 &&
				input.Divergence <= divergenceEdge

			if !laminarRegime {
				viscousScore = math.Max(viscousScore, addShare*input.Viscosity)
			}
		}

		if input.TurbulentReady && executeShare > 0 {
			turbulentScore = math.Max(turbulentScore, executeShare*input.Reynolds)
		}
	}

	category := 1
	best := laminarScore

	if turbulentScore > best {
		best = turbulentScore
		category = 2
	}

	if inertialScore > best {
		best = inertialScore
		category = 3
	}

	if viscousScore > best {
		best = viscousScore
		category = 4
	}

	if best <= 0 &&
		input.Price > 0 &&
		input.LaminarCeiling > 0 &&
		input.Reynolds <= input.LaminarCeiling {
		category = 1
		laminarScore = input.Viscosity
		best = laminarScore
	}

	strength := input.Reynolds

	if strength <= 0 || math.IsNaN(strength) || math.IsInf(strength, 0) {
		strength = math.Max(
			laminarScore,
			math.Max(turbulentScore, math.Max(inertialScore, viscousScore)),
		)
	}

	if strength <= 0 || math.IsNaN(strength) || math.IsInf(strength, 0) {
		return FluidflowOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"fluidflow: invalid strength",
			nil,
		))
	}

	return FluidflowOutput{
		Value:          strength,
		LaminarScore:   laminarScore,
		TurbulentScore: turbulentScore,
		InertialScore:  inertialScore,
		ViscousScore:   viscousScore,
		Strength:       strength,
		Category:       float64(category),
		Price:          input.Price,
		SpreadBPS:      input.SpreadBPS,
		ChangePct:      input.ChangePct,
		Volume:         input.Volume,
	}, nil
}

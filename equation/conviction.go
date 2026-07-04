package equation

import (
	"math"

	"github.com/theapemachine/errnie"
)

/*
Conviction classifies risk-on breadth versus idiosyncratic leadership.
*/
type Conviction struct{}

/*
ConvictionInput contains the float-only market breadth inputs.
*/
type ConvictionInput struct {
	Breadth        float64
	Change         float64
	SurgeThreshold float64
	Leader         bool
}

/*
ConvictionOutput contains the float-only conviction scores.
*/
type ConvictionOutput struct {
	Value          float64
	SurgeScore     float64
	DivergentScore float64
	SlumpScore     float64
	Strength       float64
	Category       float64
	Breadth        float64
	Change         float64
}

/*
NewConviction returns a market-breadth conviction calculator.
*/
func NewConviction() *Conviction {
	return &Conviction{}
}

/*
Measure calculates conviction scores from floats without artifact transport.
*/
func (conviction *Conviction) Measure(
	input ConvictionInput,
) (ConvictionOutput, error) {
	for _, value := range []float64{input.Breadth, input.Change, input.SurgeThreshold} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return ConvictionOutput{}, errnie.Error(errnie.Err(
				errnie.Validation,
				"conviction: invalid input",
				nil,
			))
		}
	}

	if input.SurgeThreshold <= 0 {
		return ConvictionOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"conviction: surgeThreshold must be positive",
			nil,
		))
	}

	surgeThreshold := input.SurgeThreshold
	if surgeThreshold > 1 {
		surgeThreshold = 1
	}

	category := conviction.classify(input.Breadth, input.Change, surgeThreshold, input.Leader)
	output := ConvictionOutput{
		Category: float64(category),
		Breadth:  input.Breadth,
		Change:   input.Change,
	}

	switch category {
	case 1:
		output.SurgeScore = input.Breadth
		output.Strength = output.SurgeScore
	case 2:
		output.DivergentScore = math.Abs(input.Change)
		output.Strength = output.DivergentScore
	case 3:
		output.SlumpScore = math.Max(math.Max(0, surgeThreshold-input.Breadth), math.Abs(input.Change))
		output.Strength = output.SlumpScore
	}

	output.Value = output.Strength

	return output, nil
}

func classifyConviction(
	breadth, change, surgeThreshold float64,
	leader bool,
) int {
	if breadth >= surgeThreshold && leader {
		return 1
	}

	if leader && change != 0 {
		return 2
	}

	return 3
}

func (conviction *Conviction) classify(
	breadth, change, surgeThreshold float64,
	leader bool,
) int {
	return classifyConviction(breadth, change, surgeThreshold, leader)
}

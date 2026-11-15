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
	Breadth         float64
	Change          float64
	SurgeThreshold  float64
	ChangeThreshold float64
	Leader          bool
}

/*
ConvictionOutput contains the float-only conviction scores.
*/
type ConvictionOutput struct {
	Value          float64 `json:"value"`
	SurgeScore     float64 `json:"surge_score"`
	DivergentScore float64 `json:"divergent_score"`
	SlumpScore     float64 `json:"slump_score"`
	Strength       float64 `json:"strength"`
	Category       float64 `json:"category"`
	Breadth        float64 `json:"breadth"`
	Change         float64 `json:"change"`
}

/*
Map returns scored labels for the grid.
*/
func (output *ConvictionOutput) Map() map[string]float64 {
	if output == nil {
		return nil
	}

	return map[string]float64{
		"surge_score":     output.SurgeScore,
		"divergent_score": output.DivergentScore,
		"slump_score":     output.SlumpScore,
		"breadth":         output.Breadth,
		"change":          output.Change,
		"value":           output.Value,
		"strength":        output.Strength,
	}
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
	for _, value := range []float64{input.Breadth, input.Change, input.SurgeThreshold, input.ChangeThreshold} {
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

	category := conviction.classify(input.Breadth, input.Change, surgeThreshold, input.ChangeThreshold)
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

/*
classifyConviction separates surge, divergent, and slump from breadth and change.
Surge is a positive change aligned with positive breadth. It does not require
being the single largest move. Breadth uses its own gate; change magnitude
uses ChangeThreshold and is never compared with the breadth fraction.
*/
func classifyConviction(
	breadth, change, breadthGate, changeGate float64,
) int {
	if change > 0 && breadth > 0 && breadth >= breadthGate {
		return 1
	}

	meaningful := changeGate <= 0 || math.Abs(change) >= changeGate

	if meaningful && change > 0 && breadth < breadthGate {
		return 2
	}

	if meaningful && change < 0 && breadth >= breadthGate {
		return 2
	}

	return 3
}

/*
classify forwards to the package classifier so Measure stays a thin adapter.
*/
func (conviction *Conviction) classify(
	breadth, change, breadthGate, changeGate float64,
) int {
	return classifyConviction(breadth, change, breadthGate, changeGate)
}

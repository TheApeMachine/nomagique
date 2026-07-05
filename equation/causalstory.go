package equation

import (
	"math"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/probability"
)

/*
CausalStoryInput carries Pearl ladder evidence into semantic category scoring.
*/
type CausalStoryInput struct {
	Association          float64
	Intervention         float64
	InterventionScore    float64
	HasInterventionScore bool
	Uplift               float64
	UpliftScore          float64
	HasUpliftScore       bool
	Contagion            float64
	Condition            float64
	Inverted             bool
}

/*
CausalStoryOutput reports semantic causal category evidence.
*/
type CausalStoryOutput struct {
	Value             float64
	Ready             bool
	AlphaScore        float64
	BetaScore         float64
	ShockScore        float64
	NoiseScore        float64
	Strength          float64
	Category          int
	Association       float64
	Intervention      float64
	InterventionScore float64
	Uplift            float64
	UpliftScore       float64
	Contagion         float64
	Condition         float64
}

/*
CausalStory maps Pearl ladder outputs into semantic category scores.
*/
type CausalStory struct{}

/*
NewCausalStory returns a typed causal semantic scorer.
*/
func NewCausalStory() *CausalStory {
	return &CausalStory{}
}

/*
Measure scores the ladder evidence into alpha, beta, shock, and noise channels.
*/
func (causalStory *CausalStory) Measure(
	input CausalStoryInput,
) (CausalStoryOutput, error) {
	interventionScore := input.InterventionScore
	upliftScore := input.UpliftScore

	if !input.HasInterventionScore {
		interventionScore = input.Intervention
	}

	if !input.HasUpliftScore {
		upliftScore = input.Uplift
	}

	if err := validateCausalStory(input, interventionScore, upliftScore); err != nil {
		return CausalStoryOutput{}, err
	}

	association := math.Abs(input.Association)
	intervention := math.Abs(interventionScore)
	uplift := math.Abs(upliftScore)
	contagion := math.Abs(input.Contagion)
	condition := math.Abs(input.Condition)
	rungTotal := association + intervention + uplift

	if rungTotal <= 0 && !input.Inverted {
		return CausalStoryOutput{}, nil
	}

	output, err := causalStory.score(input, association, intervention, uplift)
	if err != nil {
		return CausalStoryOutput{}, err
	}

	if input.Inverted {
		output.ShockScore = shockScore(contagion, condition, rungTotal)
	}

	if rungTotal > 0 {
		noise, err := noiseScore(association, intervention, uplift, rungTotal)
		if err != nil {
			return CausalStoryOutput{}, err
		}

		output.NoiseScore = noise
	}

	output = chooseCausalCategory(output)
	if !output.Ready {
		return output, nil
	}

	output.Value = output.Strength
	output.Association = association
	output.Intervention = input.Intervention
	output.InterventionScore = intervention
	output.Uplift = input.Uplift
	output.UpliftScore = uplift
	output.Contagion = contagion
	output.Condition = condition

	return output, nil
}

func (causalStory *CausalStory) score(
	input CausalStoryInput,
	association float64,
	intervention float64,
	uplift float64,
) (CausalStoryOutput, error) {
	output := CausalStoryOutput{}

	if !input.Inverted && uplift > 0 && intervention > 0 {
		margin, err := probability.CompetitionMargin(
			uplift,
			association+intervention,
		)
		if err != nil {
			return CausalStoryOutput{}, err
		}

		output.AlphaScore = margin * uplift
	}

	betaIntervention := math.Abs(input.Intervention)

	if !input.Inverted && association > betaIntervention {
		margin := association - betaIntervention
		score, err := probability.CompetitionMargin(margin, association)
		if err != nil {
			return CausalStoryOutput{}, err
		}

		output.BetaScore = score * association
	}

	return output, nil
}

func validateCausalStory(
	input CausalStoryInput,
	interventionScore float64,
	upliftScore float64,
) error {
	values := []float64{
		input.Association,
		input.Intervention,
		input.Uplift,
		interventionScore,
		upliftScore,
		input.Contagion,
		input.Condition,
	}

	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return errnie.Error(errnie.Err(
				errnie.Validation,
				"causalstory: ladder output is non-finite",
				nil,
			))
		}
	}

	return nil
}

func shockScore(contagion float64, condition float64, rungTotal float64) float64 {
	shockEvidence := contagion

	if condition > 0 && rungTotal > 0 {
		collinearity := condition / (condition + rungTotal)

		if collinearity > shockEvidence {
			shockEvidence = collinearity
		}
	}

	return shockEvidence
}

func noiseScore(
	association float64,
	intervention float64,
	uplift float64,
	rungTotal float64,
) (float64, error) {
	dominant := math.Max(association, math.Max(intervention, uplift))
	residual := rungTotal - dominant

	if residual <= 0 {
		return 0, nil
	}

	return probability.CompetitionMargin(residual, rungTotal)
}

func chooseCausalCategory(output CausalStoryOutput) CausalStoryOutput {
	scores := []float64{
		output.AlphaScore,
		output.BetaScore,
		output.ShockScore,
		output.NoiseScore,
	}
	best := math.Max(
		output.AlphaScore,
		math.Max(output.BetaScore, math.Max(output.ShockScore, output.NoiseScore)),
	)

	if best <= 0 {
		return output
	}

	category := 0
	winners := 0

	for index, score := range scores {
		if score != best {
			continue
		}

		winners++
		category = index + 1
	}

	if winners != 1 {
		return output
	}

	output.Category = category
	output.Strength = best
	output.Ready = true

	return output
}

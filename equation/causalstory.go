package equation

import (
	"io"
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/probability"
)

/*
CausalStory maps Pearl ladder outputs into the four semantic category scores.
*/
type CausalStory struct {
	artifact *datura.Artifact
}

/*
NewCausalStory returns a causal semantic scoring stage.
*/
func NewCausalStory() io.ReadWriteCloser {
	return &CausalStory{
		artifact: datura.Acquire("causal-story", datura.APPJSON),
	}
}

func (causalStory *CausalStory) Write(payload []byte) (int, error) {
	causalStory.artifact.WithPayload(payload)
	return len(payload), nil
}

func (causalStory *CausalStory) Read(payload []byte) (int, error) {
	state, err := stageState(causalStory.artifact.DecryptPayload())

	if err != nil {
		return 0, err
	}

	association := datura.Peek[float64](state, "output", "association")
	intervention := datura.Peek[float64](state, "output", "intervention")
	uplift := datura.Peek[float64](state, "output", "uplift")
	contagion := datura.Peek[float64](state, "output", "contagion")
	condition := datura.Peek[float64](state, "output", "condition")
	inverted := datura.Peek[float64](state, "output", "inverted") > 0

	if math.IsNaN(association) || math.IsInf(association, 0) {
		association = 0
	}

	if math.IsNaN(intervention) || math.IsInf(intervention, 0) {
		intervention = 0
	}

	if math.IsNaN(uplift) || math.IsInf(uplift, 0) {
		uplift = 0
	}

	if math.IsNaN(contagion) || math.IsInf(contagion, 0) {
		contagion = 0
	}

	if math.IsNaN(condition) || math.IsInf(condition, 0) {
		condition = 0
	}

	association = math.Abs(association)
	intervention = math.Abs(intervention)
	uplift = math.Abs(uplift)
	contagion = math.Abs(contagion)
	condition = math.Abs(condition)

	rungTotal := association + intervention + uplift

	if rungTotal <= 0 && !inverted {
		return emitZero(state, payload)
	}

	alphaScore := 0.0
	betaScore := 0.0
	shockScore := 0.0
	noiseScore := 0.0

	if !inverted && uplift > 0 && intervention > 0 {
		alphaScore = probability.CompetitionMargin(uplift, association+intervention) * uplift
	}

	if !inverted && association > intervention {
		margin := association - intervention
		betaScore = probability.CompetitionMargin(margin, association)
	}

	if inverted {
		shockEvidence := contagion

		if condition > 0 && rungTotal > 0 {
			collinearity := condition / (condition + rungTotal)

			if collinearity > shockEvidence {
				shockEvidence = collinearity
			}
		}

		if shockEvidence > 0 {
			shockScore = shockEvidence
		}
	}

	if rungTotal > 0 {
		dominant := math.Max(association, math.Max(intervention, uplift))
		residual := rungTotal - dominant
		noiseScore = probability.CompetitionMargin(residual, rungTotal)
	}

	best := math.Max(alphaScore, math.Max(betaScore, math.Max(shockScore, noiseScore)))

	if best <= 0 {
		return emitZero(state, payload)
	}

	category := 1

	if betaScore >= best {
		category = 2
	}

	if shockScore >= best {
		category = 3
	}

	if noiseScore >= best {
		category = 4
	}

	return emitOutput(state, payload, datura.Map[float64]{
		"value":        best,
		"alphaScore":   alphaScore,
		"betaScore":    betaScore,
		"shockScore":   shockScore,
		"noiseScore":   noiseScore,
		"strength":     best,
		"category":     float64(category),
		"association":  association,
		"intervention": intervention,
		"uplift":       uplift,
		"contagion":    contagion,
		"condition":    condition,
	})
}

func (causalStory *CausalStory) Close() error {
	return nil
}

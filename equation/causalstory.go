package equation

import (
	"fmt"
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
NewCausalStory returns a causal semantic scoring stage wired from config attributes.
*/
func NewCausalStory(artifact *datura.Artifact) io.ReadWriteCloser {
	return &CausalStory{
		artifact: artifact,
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
	rawIntervention := datura.Peek[float64](state, "output", "intervention")
	rawUplift := datura.Peek[float64](state, "output", "uplift")
	intervention := datura.Peek[float64](state, "output", "interventionScore")
	uplift := datura.Peek[float64](state, "output", "upliftScore")
	contagion := datura.Peek[float64](state, "output", "contagion")
	condition := datura.Peek[float64](state, "output", "condition")
	inverted := datura.Peek[float64](state, "output", "inverted") > 0

	if intervention == 0 {
		intervention = rawIntervention
	}

	if uplift == 0 {
		uplift = rawUplift
	}

	for _, value := range []float64{association, intervention, uplift, rawIntervention, rawUplift, contagion, condition} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return rejectStage(state, "causalstory: ladder output is non-finite")
		}
	}

	association = math.Abs(association)
	intervention = math.Abs(intervention)
	uplift = math.Abs(uplift)
	contagion = math.Abs(contagion)
	condition = math.Abs(condition)

	rungTotal := association + intervention + uplift

	if rungTotal <= 0 && !inverted {
		state.Release()

		return 0, io.EOF
	}

	alphaScore := 0.0
	betaScore := 0.0
	shockScore := 0.0
	noiseScore := 0.0

	if !inverted && uplift > 0 && intervention > 0 {
		margin, err := probability.CompetitionMargin(uplift, association+intervention)

		if err != nil {
			return rejectStage(state, fmt.Sprintf("causalstory: alpha margin failed: %v", err))
		}

		alphaScore = margin * uplift
	}

	betaIntervention := math.Abs(rawIntervention)

	if !inverted && association > betaIntervention {
		margin := association - betaIntervention
		score, err := probability.CompetitionMargin(margin, association)

		if err != nil {
			return rejectStage(state, fmt.Sprintf("causalstory: beta margin failed: %v", err))
		}

		betaScore = score * association
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

		if residual > 0 {
			score, err := probability.CompetitionMargin(residual, rungTotal)

			if err != nil {
				return rejectStage(state, fmt.Sprintf("causalstory: noise margin failed: %v", err))
			}

			noiseScore = score
		}
	}

	best := math.Max(alphaScore, math.Max(betaScore, math.Max(shockScore, noiseScore)))

	if best <= 0 {
		state.Release()

		return 0, io.EOF
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
		"value":             best,
		"alphaScore":        alphaScore,
		"betaScore":         betaScore,
		"shockScore":        shockScore,
		"noiseScore":        noiseScore,
		"strength":          best,
		"category":          float64(category),
		"association":       association,
		"intervention":      rawIntervention,
		"interventionScore": intervention,
		"uplift":            rawUplift,
		"upliftScore":       uplift,
		"contagion":         contagion,
		"condition":         condition,
	})
}

func (causalStory *CausalStory) Close() error {
	return nil
}

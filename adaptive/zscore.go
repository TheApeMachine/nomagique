package adaptive

import (
	"github.com/theapemachine/nomagique/core"
)

/*
Surprise scores how many adaptive standard deviations the sample sits from a level.
*/
type Surprise struct {
	stageParser *core.StageParser
	state       ZScoreState
}

/*
ZScore returns a z-score dynamic ready from its first observation.
*/
func ZScore() *Surprise {
	return &Surprise{
		stageParser: core.NewStageParser(),
	}
}

/*
Observe derives the z-score for the current sample.
*/
func (surprise *Surprise) Observe(
	inputs ...core.Number,
) core.Float64 {
	out, work, err := surprise.stageParser.Parse(inputs)

	if err != nil {
		return 0
	}

	result, err := surprise.Apply(out, work)

	if err != nil {
		return 0
	}

	return result
}

/*
Apply runs one pipeline stage without allocating number inputs.
*/
func (surprise *Surprise) Apply(
	out core.Float64, work []core.Float64,
) (core.Float64, error) {
	sample := float64(out)
	anchor := float64(out)
	hasAnchor := len(work) > 0

	if hasAnchor {
		sample = float64(work[0])
	}

	return core.Float64(
		ObserveZScore(&surprise.state, sample, anchor, hasAnchor),
	), nil
}

/*
ObserveSamples runs the exact batch kernel over samples into out.
*/
func (surprise *Surprise) ObserveSamples(
	samples []float64, out []float64,
) {
	surprise.state.ObserveSamples(samples, out)
}

/*
Reset clears derived state.
*/
func (surprise *Surprise) Reset() error {
	surprise.state.Reset()
	return nil
}

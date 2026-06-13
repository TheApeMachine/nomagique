package probability

import (
	"github.com/theapemachine/nomagique/core"
)

/*
EmpiricalRank tracks P(history <= current sample) over a span-derived window.
*/
type EmpiricalRank struct {
	stageParser *core.StageParser
	state       RankState
}

/*
Rank returns an empirical rank probability dynamic ready from its first observation.
*/
func Rank() *EmpiricalRank {
	return &EmpiricalRank{
		stageParser: core.NewStageParser(),
	}
}

/*
Observe derives the empirical rank probability for the current sample.
*/
func (empiricalRank *EmpiricalRank) Observe(
	inputs ...core.Number,
) core.Float64 {
	out, work, err := empiricalRank.stageParser.Parse(inputs)

	if err != nil {
		return 0
	}

	result, err := empiricalRank.Apply(out, work)

	if err != nil {
		return 0
	}

	return result
}

/*
Apply runs one pipeline stage without allocating number inputs.
*/
func (empiricalRank *EmpiricalRank) Apply(
	out core.Float64, work []core.Float64,
) (core.Float64, error) {
	sample := float64(out)

	if len(work) > 0 {
		sample = float64(out) + float64(work[0])
	}

	return core.Float64(ObserveRank(&empiricalRank.state, sample)), nil
}

/*
ObserveSamples runs the exact batch kernel over samples into out.
*/
func (empiricalRank *EmpiricalRank) ObserveSamples(samples []float64, out []float64) {
	empiricalRank.state.ObserveSamples(samples, out)
}

/*
Reset clears derived state.
*/
func (empiricalRank *EmpiricalRank) Reset() error {
	empiricalRank.state.Reset()
	return nil
}

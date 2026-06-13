package probability

import (
	"github.com/theapemachine/nomagique/core"
)

/*
ChangeSum accumulates sequential change evidence from a sample stream.
*/
type ChangeSum struct {
	stageParser *core.StageParser
	state       CUSUMState
}

/*
CUSUM returns a change-detection dynamic ready from its first observation.
*/
func CUSUM() *ChangeSum {
	return &ChangeSum{
		stageParser: core.NewStageParser(),
	}
}

/*
Observe derives cumulative change evidence for the current sample.
*/
func (changeSum *ChangeSum) Observe(
	inputs ...core.Number,
) core.Float64 {
	out, work, err := changeSum.stageParser.Parse(inputs)

	if err != nil {
		return 0
	}

	result, err := changeSum.Apply(out, work)

	if err != nil {
		return 0
	}

	return result
}

/*
Apply runs one pipeline stage without allocating number inputs.
*/
func (changeSum *ChangeSum) Apply(
	out core.Float64, work []core.Float64,
) (core.Float64, error) {
	sample := float64(out)

	if len(work) > 0 {
		sample = float64(out) + float64(work[0])
	}

	return core.Float64(ObserveCUSUM(&changeSum.state, sample)), nil
}

/*
ObserveSamples runs the exact batch kernel over samples into out.
*/
func (changeSum *ChangeSum) ObserveSamples(samples []float64, out []float64) {
	changeSum.state.ObserveSamples(samples, out)
}

/*
Reset clears derived state.
*/
func (changeSum *ChangeSum) Reset() error {
	changeSum.state.Reset()
	return nil
}

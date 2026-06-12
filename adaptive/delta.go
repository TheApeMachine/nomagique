package adaptive

import (
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/kernel"
)

/*
Normalized tracks a unit-normalized change of the observed sample relative to its range.
*/
type Normalized struct {
	stageParser *core.StageParser
	state       kernel.DeltaState
}

/*
Delta returns a delta dynamic ready to bootstrap from its first observation.
*/
func Delta() *Normalized {
	return &Normalized{
		stageParser: core.NewStageParser(),
	}
}

/*
Observe derives the normalized delta for the current sample.
*/
func (delta *Normalized) Observe(
	inputs ...core.Number,
) core.Float64 {
	out, work, err := delta.stageParser.Parse(inputs)

	if err != nil {
		return 0
	}

	result, err := delta.Apply(out, work)

	if err != nil {
		return 0
	}

	return result
}

/*
Apply runs one pipeline stage without allocating number inputs.
*/
func (delta *Normalized) Apply(
	out core.Float64, work []core.Float64,
) (core.Float64, error) {
	sample := float64(out)

	if len(work) > 0 {
		sample = float64(work[0])
	}

	return core.Float64(delta.state.Observe(sample)), nil
}

/*
ObserveSamples runs the exact batch kernel over samples into out.
*/
func (delta *Normalized) ObserveSamples(
	samples []float64, out []float64,
) {
	delta.state.ObserveSamples(samples, out)
}

/*
Reset clears derived state so the next Observe bootstraps again.
*/
func (delta *Normalized) Reset() error {
	delta.state.Reset()
	return nil
}

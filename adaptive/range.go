package adaptive

import (
	"github.com/theapemachine/nomagique/core"
)

/*
Extent tracks the running span between minimum and maximum observations.
*/
type Extent struct {
	stageParser *core.StageParser
	state       RangeState
}

/*
Range returns a range dynamic ready from its first observation.
*/
func Range() *Extent {
	return &Extent{
		stageParser: core.NewStageParser(),
	}
}

/*
Observe derives the current range span for the sample stream.
*/
func (extent *Extent) Observe(
	inputs ...core.Number,
) core.Float64 {
	out, work, err := extent.stageParser.Parse(inputs)

	if err != nil {
		return 0
	}

	result, err := extent.Apply(out, work)

	if err != nil {
		return 0
	}

	return result
}

/*
Apply runs one pipeline stage without allocating number inputs.
*/
func (extent *Extent) Apply(
	out core.Float64, work []core.Float64,
) (core.Float64, error) {
	sample := float64(out)

	if len(work) > 0 {
		sample = float64(out) + float64(work[0])
	}

	return core.Float64(extent.state.Observe(sample)), nil
}

/*
ObserveSamples runs the exact batch kernel over samples into out.
*/
func (extent *Extent) ObserveSamples(
	samples []float64, out []float64,
) {
	extent.state.ObserveSamples(samples, out)
}

/*
Reset clears derived state.
*/
func (extent *Extent) Reset() error {
	extent.state.Reset()
	return nil
}

package adaptive

import (
	"github.com/theapemachine/nomagique/core"
)

/*
Impulse tracks signed normalized momentum relative to the running range.
*/
type Impulse struct {
	stageParser *core.StageParser
	state       MomentumState
}

/*
Momentum returns a signed momentum dynamic ready from its first observation.
*/
func Momentum() *Impulse {
	return &Impulse{
		stageParser: core.NewStageParser(),
	}
}

/*
Observe derives signed normalized momentum for the current sample.
*/
func (impulse *Impulse) Observe(
	inputs ...core.Number,
) core.Float64 {
	out, work, err := impulse.stageParser.Parse(inputs)

	if err != nil {
		return 0
	}

	result, err := impulse.Apply(out, work)

	if err != nil {
		return 0
	}

	return result
}

/*
Apply runs one pipeline stage without allocating number inputs.
*/
func (impulse *Impulse) Apply(
	out core.Float64, work []core.Float64,
) (core.Float64, error) {
	sample := float64(out)

	if len(work) > 0 {
		sample = float64(out) + float64(work[0])
	}

	return core.Float64(impulse.state.Observe(sample)), nil
}

/*
ObserveSamples runs the exact batch kernel over samples into out.
*/
func (impulse *Impulse) ObserveSamples(
	samples []float64, out []float64,
) {
	impulse.state.ObserveSamples(samples, out)
}

/*
Reset clears derived state.
*/
func (impulse *Impulse) Reset() error {
	impulse.state.Reset()
	return nil
}

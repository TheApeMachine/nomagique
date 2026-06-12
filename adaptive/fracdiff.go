package adaptive

import (
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/kernel"
)

/*
Fractional applies a fixed-width fractional differencing filter to recent samples.
*/
type Fractional struct {
	stageParser *core.StageParser
	state       kernel.FracDiffState
}

/*
FracDiff returns a fractional differencing filter ready from its first observation.
*/
func FracDiff() *Fractional {
	return &Fractional{
		stageParser: core.NewStageParser(),
	}
}

/*
Observe derives the fractionally differenced value for the current sample.
*/
func (fractional *Fractional) Observe(
	inputs ...core.Number,
) core.Float64 {
	out, work, err := fractional.stageParser.Parse(inputs)

	if err != nil {
		return 0
	}

	result, err := fractional.Apply(out, work)

	if err != nil {
		return 0
	}

	return result
}

/*
Apply runs one pipeline stage without allocating number inputs.
*/
func (fractional *Fractional) Apply(
	out core.Float64, work []core.Float64,
) (core.Float64, error) {
	sample := float64(out)

	if len(work) > 0 {
		sample = float64(work[0])
	}

	return core.Float64(fractional.state.Observe(sample)), nil
}

/*
ObserveSamples runs the exact batch kernel over samples into out.
*/
func (fractional *Fractional) ObserveSamples(
	samples []float64, out []float64,
) {
	fractional.state.ObserveSamples(samples, out)
}

/*
Reset clears derived state.
*/
func (fractional *Fractional) Reset() error {
	fractional.state.Reset()
	return nil
}

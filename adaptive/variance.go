package adaptive

import (
	"github.com/theapemachine/nomagique/core"
)

/*
Dispersion tracks adaptive variance of the observed sample stream.
*/
type Dispersion struct {
	stageParser *core.StageParser
	state       VarianceState
}

/*
Variance returns a variance dynamic ready from its first observation.
*/
func Variance() *Dispersion {
	return &Dispersion{
		stageParser: core.NewStageParser(),
	}
}

/*
Observe derives the current variance for the sample stream.
*/
func (dispersion *Dispersion) Observe(
	inputs ...core.Number,
) core.Float64 {
	out, work, err := dispersion.stageParser.Parse(inputs)

	if err != nil {
		return 0
	}

	result, err := dispersion.Apply(out, work)

	if err != nil {
		return 0
	}

	return result
}

/*
Apply runs one pipeline stage without allocating number inputs.
*/
func (dispersion *Dispersion) Apply(
	out core.Float64, work []core.Float64,
) (core.Float64, error) {
	sample := float64(out)

	if len(work) > 0 {
		sample = float64(out) + float64(work[0])
	}

	return core.Float64(dispersion.state.Observe(sample)), nil
}

/*
ObserveSamples runs the exact batch kernel over samples into out.
*/
func (dispersion *Dispersion) ObserveSamples(
	samples []float64, out []float64,
) {
	dispersion.state.ObserveSamples(samples, out)
}

/*
Reset clears derived state.
*/
func (dispersion *Dispersion) Reset() error {
	dispersion.state.Reset()
	return nil
}

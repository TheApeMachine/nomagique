/*
Package adaptive provides dynamics whose parameters emerge from the observed signal.
Hot-path kernels and pipeline bindings for adaptive signal dynamics.
*/
package adaptive

import (
	"github.com/theapemachine/nomagique/core"
)

/*
Exponential is a volatility-adaptive exponential moving average.
*/
type Exponential struct {
	stageParser *core.StageParser
	state       EMAState
}

/*
EMA returns an exponential dynamic ready to bootstrap from its first observation.
*/
func EMA() *Exponential {
	return &Exponential{
		stageParser: core.NewStageParser(),
	}
}

/*
Observe absorbs a pipeline stage input and evolves the EMA.
*/
func (exponential *Exponential) Observe(
	inputs ...core.Number,
) core.Float64 {
	out, work, err := exponential.stageParser.Parse(inputs)

	if err != nil {
		return 0
	}

	result, err := exponential.Apply(out, work)

	if err != nil {
		return 0
	}

	return result
}

/*
Apply runs one pipeline stage without allocating number inputs.
*/
func (exponential *Exponential) Apply(
	out core.Float64, work []core.Float64,
) (core.Float64, error) {
	sample := float64(out)

	if len(work) > 0 {
		sample = float64(out) + float64(work[0])
	}

	return core.Float64(exponential.state.Observe(sample)), nil
}

/*
ObserveSamples runs the exact batch kernel over samples into out.
*/
func (exponential *Exponential) ObserveSamples(
	samples []float64, out []float64,
) {
	exponential.state.ObserveSamples(samples, out)
}

/*
Reset clears all derived state so the next Observe bootstraps again from a fresh sample.
*/
func (exponential *Exponential) Reset() error {
	exponential.state.Reset()
	return nil
}

/*
Package adaptive provides dynamics whose parameters emerge from the observed signal.
Algorithms live in kernel; this package wires them into core.Number pipelines.
*/
package adaptive

import (
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/kernel"
)

/*
Integrator is a capacitor that integrates signed signal strength with no bounds.
*/
type Integrator struct {
	stageParser *core.StageParser
	state       kernel.AccumulatorState
}

/*
Accumulator returns an integrator ready from its first observation.
*/
func Accumulator() *Integrator {
	return &Integrator{
		stageParser: core.NewStageParser(),
	}
}

/*
Observe absorbs a pipeline stage input and updates the integrated level.
*/
func (integrator *Integrator) Observe(
	inputs ...core.Number,
) core.Float64 {
	out, work, err := integrator.stageParser.Parse(inputs)

	if err != nil {
		return 0
	}

	result, err := integrator.Apply(out, work)

	if err != nil {
		return 0
	}

	return result
}

/*
Apply runs one pipeline stage without allocating number inputs.
*/
func (integrator *Integrator) Apply(
	out core.Float64, work []core.Float64,
) (core.Float64, error) {
	sample := float64(out)

	if len(work) > 0 {
		sample = float64(out) + float64(work[0])
	}

	return core.Float64(integrator.state.Observe(sample)), nil
}

/*
ObserveSamples runs the exact batch kernel over samples into out.
*/
func (integrator *Integrator) ObserveSamples(
	samples []float64, out []float64,
) {
	integrator.state.ObserveSamples(samples, out)
}

/*
Reset clears derived state.
*/
func (integrator *Integrator) Reset() error {
	integrator.state.Reset()
	return nil
}

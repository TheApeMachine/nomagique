/*
Package geometric wires geometry phase dynamics into core.Number pipelines.
*/
package geometric

import (
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/kernel/geom"
)

/*
PhaseVelocity tracks surprisal mean velocity between consecutive observations.
*/
type PhaseVelocity struct {
	stageParser *core.StageParser
	state       geom.PhaseVelocityState
}

/*
Velocity returns a phase-velocity dynamic ready from its first observation.
*/
func Velocity() *PhaseVelocity {
	return &PhaseVelocity{
		stageParser: core.NewStageParser(),
	}
}

/*
Observe ingests a mean sample and returns its velocity versus the previous mean.
*/
func (phaseVelocity *PhaseVelocity) Observe(
	inputs ...core.Number,
) core.Float64 {
	out, work, err := phaseVelocity.stageParser.Parse(inputs)

	if err != nil {
		return 0
	}

	result, err := phaseVelocity.Apply(out, work)

	if err != nil {
		return 0
	}

	return result
}

/*
Apply runs one pipeline stage without allocating number inputs.
*/
func (phaseVelocity *PhaseVelocity) Apply(
	out core.Float64, work []core.Float64,
) (core.Float64, error) {
	mean := float64(out)

	if len(work) > 0 {
		mean = float64(out) + float64(work[0])
	}

	return core.Float64(phaseVelocity.state.Observe(mean)), nil
}

/*
ObserveSamples runs the exact batch kernel over means into out.
*/
func (phaseVelocity *PhaseVelocity) ObserveSamples(means []float64, out []float64) {
	phaseVelocity.state.ObserveSamples(means, out)
}

/*
Reset clears derived state.
*/
func (phaseVelocity *PhaseVelocity) Reset() error {
	phaseVelocity.state.Reset()
	return nil
}

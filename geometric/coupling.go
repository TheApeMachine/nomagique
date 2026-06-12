package geometric

import (
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/geometry"
)

/*
PhaseCoupling measures directional alignment of two growth signals in [-1, +1].
*/
type PhaseCoupling struct {
	stageParser *core.StageParser
	phase       *geometry.Phase
}

/*
Coupling returns a phase-coupling dynamic.
*/
func Coupling() *PhaseCoupling {
	return &PhaseCoupling{
		stageParser: core.NewStageParser(),
		phase:       geometry.NewPhase(),
	}
}

/*
Observe ingests left and right growth values and returns coupling strength.
*/
func (phaseCoupling *PhaseCoupling) Observe(
	inputs ...core.Number,
) core.Float64 {
	out, work, err := phaseCoupling.stageParser.Parse(inputs)

	if err != nil {
		return 0
	}

	result, err := phaseCoupling.Apply(out, work)

	if err != nil {
		return 0
	}

	return result
}

/*
Apply runs one pipeline stage without allocating number inputs.
*/
func (phaseCoupling *PhaseCoupling) Apply(
	out core.Float64, work []core.Float64,
) (core.Float64, error) {
	leftGrowth, rightGrowth, err := parseGrowthPair(out, work)

	if err != nil {
		return 0, err
	}

	return core.Float64(
		phaseCoupling.phase.Coupling(leftGrowth, rightGrowth),
	), nil
}

/*
Reset is a no-op; coupling is stateless per observation.
*/
func (phaseCoupling *PhaseCoupling) Reset() error {
	return nil
}

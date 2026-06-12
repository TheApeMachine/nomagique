package learning

import (
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/kernel/learn"
)

/*
TrustWeight is a self-adapting rate from prediction error.
*/
type TrustWeight struct {
	stageParser *core.StageParser
	state       learn.WeightState
}

/*
Weight returns a trust weight dynamic ready from its first observation.
*/
func Weight() *TrustWeight {
	return &TrustWeight{
		stageParser: core.NewStageParser(),
	}
}

/*
Observe ingests predicted and actual values and returns trust.
*/
func (trustWeight *TrustWeight) Observe(
	inputs ...core.Number,
) core.Float64 {
	out, work, err := trustWeight.stageParser.Parse(inputs)

	if err != nil {
		return 0
	}

	result, err := trustWeight.Apply(out, work)

	if err != nil {
		return 0
	}

	return result
}

/*
Apply runs one pipeline stage without allocating number inputs.
*/
func (trustWeight *TrustWeight) Apply(
	out core.Float64, work []core.Float64,
) (core.Float64, error) {
	predicted, actual, err := parsePredictedActual(out, work)

	if err != nil {
		return 0, err
	}

	return core.Float64(
		learn.ObserveWeight(&trustWeight.state, predicted, actual),
	), nil
}

/*
ObserveSamples runs the exact batch kernel over pairs into out.
*/
func (trustWeight *TrustWeight) ObserveSamples(
	predicted []float64, actual []float64, out []float64,
) {
	trustWeight.state.ObserveSamples(predicted, actual, out)
}

/*
Reset clears derived state.
*/
func (trustWeight *TrustWeight) Reset() error {
	trustWeight.state.Reset()
	return nil
}

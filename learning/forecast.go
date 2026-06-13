package learning

import (
	"github.com/theapemachine/nomagique/core"
)

/*
Forecaster learns a multiplicative scale from settled predicted-vs-actual outcomes.
*/
type Forecaster struct {
	stageParser *core.StageParser
	state       ForecastState
}

/*
Forecast returns a scale-learning dynamic ready from its first observation.
*/
func Forecast() *Forecaster {
	return &Forecaster{
		stageParser: core.NewStageParser(),
	}
}

/*
Scale returns the current multiplicative scale for parameter feedback.
*/
func (forecaster *Forecaster) Scale() float64 {
	return forecaster.state.Scale
}

/*
Observe updates scale from a predicted and actual pair.
*/
func (forecaster *Forecaster) Observe(
	inputs ...core.Number,
) core.Float64 {
	out, work, err := forecaster.stageParser.Parse(inputs)

	if err != nil {
		return 0
	}

	result, err := forecaster.Apply(out, work)

	if err != nil {
		return 0
	}

	return result
}

/*
Apply runs one pipeline stage without allocating number inputs.
*/
func (forecaster *Forecaster) Apply(
	out core.Float64, work []core.Float64,
) (core.Float64, error) {
	predicted, actual, err := parsePredictedActual(out, work)

	if err != nil {
		return 0, err
	}

	return core.Float64(
		ObserveForecast(&forecaster.state, predicted, actual),
	), nil
}

/*
ObserveSamples runs the exact batch kernel over pairs into out.
*/
func (forecaster *Forecaster) ObserveSamples(
	predicted []float64, actual []float64, out []float64,
) {
	forecaster.state.ObserveSamples(predicted, actual, out)
}

/*
Reset clears derived state.
*/
func (forecaster *Forecaster) Reset() error {
	forecaster.state.Reset()
	return nil
}

package learning

import (
	"github.com/theapemachine/nomagique/core"
)

/*
Calibrator maps predicted and actual pairs into calibration samples.
*/
type Calibrator struct {
	stageParser *core.StageParser
	state       SampleRatioState
}

/*
SampleRatio returns a calibration dynamic ready from its first observation.
*/
func SampleRatio() *Calibrator {
	return &Calibrator{
		stageParser: core.NewStageParser(),
	}
}

/*
Observe derives the calibration sample for a predicted and actual pair.
*/
func (calibrator *Calibrator) Observe(
	inputs ...core.Number,
) core.Float64 {
	out, work, err := calibrator.stageParser.Parse(inputs)

	if err != nil {
		return 0
	}

	result, err := calibrator.Apply(out, work)

	if err != nil {
		return 0
	}

	return result
}

/*
Apply runs one pipeline stage without allocating number inputs.
*/
func (calibrator *Calibrator) Apply(
	out core.Float64, work []core.Float64,
) (core.Float64, error) {
	predicted, actual, err := parsePredictedActual(out, work)

	if err != nil {
		return 0, err
	}

	return core.Float64(
		ObserveSampleRatio(&calibrator.state, predicted, actual),
	), nil
}

/*
ObserveSamples runs the exact batch kernel over pairs into out.
*/
func (calibrator *Calibrator) ObserveSamples(
	predicted []float64, actual []float64, out []float64,
) {
	calibrator.state.ObserveSamples(predicted, actual, out)
}

/*
Reset clears derived state.
*/
func (calibrator *Calibrator) Reset() error {
	calibrator.state.Reset()
	return nil
}

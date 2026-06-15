package learning

import (
	"github.com/theapemachine/datura"
)

/*
Forecaster learns a multiplicative scale from settled predicted-vs-actual outcomes.
*/
type Forecaster struct {
	artifact *datura.Artifact
	state    ForecastState
}

/*
Forecast returns a scale-learning dynamic ready from its first observation.
*/
func Forecast() *Forecaster {
	return &Forecaster{
		artifact: datura.Acquire("forecast", datura.Artifact_Type_json),
	}
}

func (forecaster *Forecaster) Write(p []byte) (int, error) {
	return forecaster.artifact.Write(p)
}

func (forecaster *Forecaster) Read(p []byte) (int, error) {
	values := float64Batch(forecaster.artifact)

	if len(values) >= 2 {
		predicted, actual, err := parsePredictedActual(values[0], values[1:])

		if err == nil {
			derived := ObserveForecast(&forecaster.state, predicted, actual)
			putFloat64Payload(&forecaster.artifact, "forecast", derived)
		}
	}

	return forecaster.artifact.Read(p)
}

func (forecaster *Forecaster) Close() error {
	return nil
}

/*
Scale returns the current multiplicative scale for parameter feedback.
*/
func (forecaster *Forecaster) Scale() float64 {
	return forecaster.state.Scale
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

package learning

import (
	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Forecaster learns a multiplicative scale from settled predicted-vs-actual outcomes.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Forecaster struct {
	artifact *datura.Artifact
}

/*
Forecast returns a scale-learning stage wired from config attributes on the artifact.
*/
func Forecast(artifact *datura.Artifact) *Forecaster {
	return &Forecaster{
		artifact: artifact,
	}
}

func (forecaster *Forecaster) Read(payload []byte) (int, error) {
	state := datura.Acquire("forecast-state", datura.APPJSON)
	
	if _, err := state.Write(forecaster.artifact.DecryptPayload()); err != nil {
		state.Release()
		
		return 0, err
	}
	
	state.Inspect("learning", "forecast", "Read()", "p")
	defer state.Release()

	predicted, actual, err := forecaster.resolvePair(state)

	if err != nil {
		return 0, err
	}

	forecastState := forecastStateFromArtifact(forecaster.artifact)
	derived := ObserveForecast(&forecastState, predicted, actual)
	pokeForecastState(forecaster.artifact, &forecastState, derived)
	state.MergeOutput("value", derived)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")
	return state.Read(payload)
}

func (forecaster *Forecaster) resolvePair(state *datura.Artifact) (float64, float64, error) {
	parsedPredicted, parsedActual, err := wirePair(forecaster.artifact, state, "forecast")

	if err != nil {
		return 0, 0, err
	}

	if parsedActual == 0 {
		return 0, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"forecast: actual must be non-zero",
			nil,
		))
	}

	return parsedPredicted, parsedActual, nil
}

func (forecaster *Forecaster) Write(payload []byte) (int, error) {
	forecaster.artifact.WithPayload(payload)
	return len(payload), nil
}

func (forecaster *Forecaster) Close() error {
	return nil
}

/*
Scale returns the current multiplicative scale for parameter feedback.
*/
func (forecaster *Forecaster) Scale() float64 {
	return datura.Peek[float64](forecaster.artifact, "output", "scale")
}

/*
ObserveSamples runs the exact batch kernel over pairs into out.
*/
func (forecaster *Forecaster) ObserveSamples(
	predicted []float64, actual []float64, out []float64,
) {
	forecastState := forecastStateFromArtifact(forecaster.artifact)
	observeForecastSamples(&forecastState, predicted, actual, out)

	if len(out) > 0 {
		pokeForecastState(forecaster.artifact, &forecastState, out[len(out)-1])
	}
}

/*
Reset clears derived state.
*/
func (forecaster *Forecaster) Reset() error {
	forecaster.artifact.Poke(0.0, "output", "scale")
	forecaster.artifact.Poke(0.0, "output", "trust")
	forecaster.artifact.Poke(0.0, "output", "prev")
	forecaster.artifact.Poke(0.0, "output", "min")
	forecaster.artifact.Poke(0.0, "output", "max")
	forecaster.artifact.Poke(0.0, "output", "rate")
	forecaster.artifact.Poke(0.0, "output", "weightReady")
	forecaster.artifact.Poke(0.0, "output", "ready")
	forecaster.artifact.Poke(0.0, "output", "value")

	return nil
}

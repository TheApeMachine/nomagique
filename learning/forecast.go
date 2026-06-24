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

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"forecast: state write failed",
			err,
		))
	}

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
ForecastState learns a multiplicative scale from predicted and actual outcomes.
*/
type ForecastState struct {
	Scale  float64
	Weight WeightState
	Ready  bool
}

/*
Observe ingests a predicted and actual pair and returns the current scale.
*/
func (state *ForecastState) Observe(predicted float64, actual float64) float64 {
	return ObserveForecast(state, predicted, actual)
}

/*
ObserveSamples writes one scale value per pair into out.
*/
func (state *ForecastState) ObserveSamples(
	predicted []float64, actual []float64, out []float64,
) {
	observeForecastSamples(state, predicted, actual, out)
}

/*
Reset clears derived state.
*/
func (state *ForecastState) Reset() {
	state.Scale = 0
	state.Weight.Reset()
	state.Ready = false
}

/*
ObserveForecast updates scale from settled predicted-vs-actual outcomes.
*/
func ObserveForecast(state *ForecastState, predicted float64, actual float64) float64 {
	if !state.Ready {
		state.Scale = 1
		state.Weight.Ready = false
		_ = ObserveWeight(&state.Weight, predicted, actual)
		state.Ready = true
		return state.Scale
	}

	return observeForecastReady(state, predicted, actual)
}

/*
observeForecastReady runs the hot forecast path; state must already be Ready.
*/
func observeForecastReady(
	state *ForecastState, predicted float64, actual float64,
) float64 {
	trust := ObserveWeight(&state.Weight, predicted, actual)
	surprise := state.Weight.Rate
	learningRate := surprise * (1 - trust)

	if predicted == 0 {
		return state.Scale
	}

	targetScale := actual / predicted
	state.Scale += learningRate * (targetScale - state.Scale)

	return state.Scale
}

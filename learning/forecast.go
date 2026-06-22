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
	state    ForecastState
}

/*
Forecast returns a scale-learning stage wired from config attributes on the artifact.
*/
func Forecast(artifact *datura.Artifact) *Forecaster {
	artifact.Inspect("learning", "forecast", "Forecast()")

	return &Forecaster{
		artifact: artifact,
	}
}

func (forecaster *Forecaster) Write(payload []byte) (int, error) {
	forecaster.artifact.WithPayload(payload)
	return len(payload), nil
}

func (forecaster *Forecaster) Read(payload []byte) (int, error) {
	state := datura.Acquire("forecast-state", datura.APPJSON)
	state.Inspect("learning", "forecast", "Read()", "p")

	if _, err := state.Write(forecaster.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	predicted := datura.Peek[float64](state, "sample")
	actual := datura.Peek[float64](state, "paired")

	if predicted == 0 && actual == 0 {
		features := datura.Peek[[]float64](state, "features")

		if len(features) >= 2 {
			predicted = features[0]
			actual = features[1]
		}
	}

	if predicted == 0 && actual == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"forecast: predicted and actual required",
			nil,
		))
	}

	if actual == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"forecast: actual must be non-zero",
			nil,
		))
	}

	parsedPredicted, parsedActual, err := parsePredictedActual(predicted, []float64{actual})

	if err != nil {
		return 0, err
	}

	derived := ObserveForecast(&forecaster.state, parsedPredicted, parsedActual)
	forecaster.artifact.Poke(derived, "output", "value")
	state.MergeOutput("value", derived)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(payload)
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
	forecaster.artifact.WithAttributes(datura.Map[any]{})

	return nil
}

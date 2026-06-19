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
		artifact: datura.Acquire("forecast", datura.APPJSON).RetainStageAttributes(),
	}
}

func (forecaster *Forecaster) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](forecaster.artifact, "output") == nil

	forecaster.artifact.Clear("sample")
	forecaster.artifact.Clear("paired")

	n, err := forecaster.artifact.Write(p)

	if bootstrap {
		forecaster.artifact.Clear("output")
	}

	return n, err
}

func (forecaster *Forecaster) Read(p []byte) (int, error) {
	predicted := datura.Peek[float64](forecaster.artifact, "sample")
	actual := datura.Peek[float64](forecaster.artifact, "paired")

	if predicted == 0 && actual == 0 {
		return forecaster.artifact.Read(p)
	}

	if actual == 0 {
		return forecaster.artifact.Read(p)
	}

	parsedPredicted, parsedActual, err := parsePredictedActual(predicted, []float64{actual})

	if err != nil {
		return forecaster.artifact.Read(p)
	}

	derived := ObserveForecast(&forecaster.state, parsedPredicted, parsedActual)
	forecaster.artifact.Poke(datura.Map[float64]{"value": derived}, "output")

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
	forecaster.artifact.Clear("output")

	return nil
}

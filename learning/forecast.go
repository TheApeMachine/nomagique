package learning

import (
	"github.com/theapemachine/nomagique/core"
)

/*
Forecaster learns a multiplicative scale from settled predicted-vs-actual outcomes.
*/
type Forecaster[T ~float64] struct {
	state  ForecastState
	output core.Scalar[T]
}

/*
Forecast returns a scale-learning dynamic ready from its first observation.
*/
func Forecast[T ~float64]() *Forecaster[T] {
	return &Forecaster[T]{}
}

/*
Scale returns the current multiplicative scale for parameter feedback.
*/
func (forecaster *Forecaster[T]) Scale() float64 {
	return forecaster.state.Scale
}

/*
Observe updates scale from a predicted and actual pair.
*/
func (forecaster *Forecaster[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if len(inputs) == 0 {
		return forecaster.output
	}

	scalars, ok := collectScalars[T](inputs...)

	if !ok {
		return forecaster.output
	}

	if len(scalars) < 2 {
		return forecaster.output
	}

	predicted, actual, err := parsePredictedActual(scalars[0], scalars[1:])

	if err != nil {
		return forecaster.output
	}

	forecaster.output = core.Scalar[T](T(
		ObserveForecast(&forecaster.state, predicted, actual),
	))

	return forecaster.output
}

/*
ObserveSamples runs the exact batch kernel over pairs into out.
*/
func (forecaster *Forecaster[T]) ObserveSamples(
	predicted []float64, actual []float64, out []float64,
) {
	forecaster.state.ObserveSamples(predicted, actual, out)
}

/*
Reset clears derived state.
*/
func (forecaster *Forecaster[T]) Reset() error {
	forecaster.state.Reset()
	forecaster.output = core.Scalar[T](0)

	return nil
}

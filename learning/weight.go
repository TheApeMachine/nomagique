package learning

import (
	"github.com/theapemachine/nomagique/core"
)

/*
TrustWeight is a self-adapting rate from prediction error.
*/
type TrustWeight[T ~float64] struct {
	state  WeightState
	output core.Scalar[T]
}

/*
Weight returns a trust weight dynamic ready from its first observation.
*/
func Weight[T ~float64]() *TrustWeight[T] {
	return &TrustWeight[T]{}
}

/*
Observe ingests predicted and actual values and returns trust.
*/
func (trustWeight *TrustWeight[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if len(inputs) == 0 {
		return trustWeight.output
	}

	scalars, ok := collectScalars[T](inputs...)

	if !ok {
		return trustWeight.output
	}

	if len(scalars) < 2 {
		return trustWeight.output
	}

	predicted, actual, err := parsePredictedActual(scalars[0], scalars[1:])

	if err != nil {
		return trustWeight.output
	}

	trustWeight.output = core.Scalar[T](T(
		ObserveWeight(&trustWeight.state, predicted, actual),
	))

	return trustWeight.output
}

/*
ObserveSamples runs the exact batch kernel over pairs into out.
*/
func (trustWeight *TrustWeight[T]) ObserveSamples(
	predicted []float64, actual []float64, out []float64,
) {
	trustWeight.state.ObserveSamples(predicted, actual, out)
}

/*
Reset clears derived state.
*/
func (trustWeight *TrustWeight[T]) Reset() error {
	trustWeight.state.Reset()
	trustWeight.output = core.Scalar[T](0)

	return nil
}

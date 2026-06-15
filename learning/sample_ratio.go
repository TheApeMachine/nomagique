package learning

import (
	"github.com/theapemachine/nomagique/core"
)

/*
Calibrator maps predicted and actual pairs into calibration samples.
*/
type Calibrator[T ~float64] struct {
	state  SampleRatioState
	output core.Scalar[T]
}

/*
SampleRatio returns a calibration dynamic ready from its first observation.
*/
func SampleRatio[T ~float64]() *Calibrator[T] {
	return &Calibrator[T]{}
}

/*
Observe derives the calibration sample for a predicted and actual pair.
*/
func (calibrator *Calibrator[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if len(inputs) == 0 {
		return calibrator.output
	}

	scalars, ok := collectScalars[T](inputs...)

	if !ok {
		return calibrator.output
	}

	if len(scalars) < 2 {
		return calibrator.output
	}

	predicted, actual, err := parsePredictedActual(scalars[0], scalars[1:])

	if err != nil {
		return calibrator.output
	}

	calibrator.output = core.Scalar[T](T(
		ObserveSampleRatio(&calibrator.state, predicted, actual),
	))

	return calibrator.output
}

/*
ObserveSamples runs the exact batch kernel over pairs into out.
*/
func (calibrator *Calibrator[T]) ObserveSamples(
	predicted []float64, actual []float64, out []float64,
) {
	calibrator.state.ObserveSamples(predicted, actual, out)
}

/*
Reset clears derived state.
*/
func (calibrator *Calibrator[T]) Reset() error {
	calibrator.state.Reset()
	calibrator.output = core.Scalar[T](0)

	return nil
}

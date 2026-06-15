package algorithm

import (
	"math"

	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/learning"
)

/*
Trust combines forecast scale calibration with adaptive prediction trust.
*/
type Trust[T ~float64] struct {
	forecaster *learning.Forecaster[T]
	weight     *learning.TrustWeight[T]
	lastTrust  float64
	output     core.Scalar[T]
}

/*
NewTrust creates a calibration-trust dynamic over predicted-vs-actual pairs.
*/
func NewTrust[T ~float64]() *Trust[T] {
	return &Trust[T]{
		forecaster: learning.Forecast[T](),
		weight:     learning.Weight[T](),
	}
}

/*
Observe ingests a predicted and actual pair and returns trust-weighted calibration.
*/
func (trust *Trust[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if len(inputs) == 0 {
		return trust.output
	}

	scalars, ok := collectScalars[T](inputs...)

	if !ok {
		return trust.output
	}

	if len(scalars) < 2 {
		return trust.output
	}

	predicted, actual, err := parsePredictedActual(scalars[0], scalars[1:])

	if err != nil {
		return trust.output
	}

	pair := samplesToInputs[T]([]float64{predicted, actual})

	_ = trust.forecaster.Observe(pair...)
	trustValue := trust.weight.Observe(pair...)
	trust.lastTrust = float64(trustValue)

	scale := trust.forecaster.Scale()
	calibration := 1 - math.Abs(1-scale)

	if calibration < 0 {
		calibration = 0
	}

	trust.output = core.Scalar[T](T(float64(trustValue) * calibration))

	return trust.output
}

/*
Scale returns the current forecast scale.
*/
func (trust *Trust[T]) Scale() float64 {
	return trust.forecaster.Scale()
}

/*
Weight returns the current adaptive trust weight.
*/
func (trust *Trust[T]) Weight() core.Scalar[T] {
	return core.Scalar[T](T(trust.lastTrust))
}

/*
Reset clears derived state.
*/
func (trust *Trust[T]) Reset() error {
	trust.lastTrust = 0
	trust.output = core.Scalar[T](0)

	if err := trust.forecaster.Reset(); err != nil {
		return err
	}

	return trust.weight.Reset()
}

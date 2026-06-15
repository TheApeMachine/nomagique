package statistic

import (
	"math"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/core"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/stat"
)

/*
Entropy computes Shannon entropy of a normalized mass distribution.
Non-positive masses are floored before normalization.
*/
type Entropy[T ~float64] struct {
	floor  float64
	output core.Scalar[T]
}

/*
NewEntropy creates an entropy stage.
floor may be zero to derive a per-sample floor from each observation.
*/
func NewEntropy[T ~float64](floor float64) *Entropy[T] {
	return &Entropy[T]{
		floor: floor,
	}
}

/*
Observe returns entropy of the normalized input masses.
*/
func (entropy *Entropy[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	values := sampleBatch[T](inputs...)

	if len(values) == 0 {
		return entropy.output
	}

	probabilities, ok := entropy.normalizedProbabilities(values)

	if !ok {
		errnie.Err(
			errnie.Validation, "unable to compute entropy",
			EntropyError(EntropyErrorNonFiniteMass),
		)

		return entropy.output
	}

	entropy.output = core.Scalar[T](T(stat.Entropy(probabilities)))

	return entropy.output
}

func (entropy *Entropy[T]) Reset() error {
	entropy.output = core.Scalar[T](0)

	return nil
}

func (entropy *Entropy[T]) normalizedProbabilities(values []float64) ([]float64, bool) {
	floor := entropy.probabilityFloor(values)
	total := 0.0

	for index := range values {
		if math.IsNaN(values[index]) || math.IsInf(values[index], 0) {
			return nil, false
		}

		mass := values[index]

		if mass < floor {
			mass = floor
		}

		values[index] = mass
		total += mass
	}

	if total <= 0 || math.IsNaN(total) || math.IsInf(total, 0) {
		return nil, false
	}

	probabilities := make([]float64, len(values))

	for index := range values {
		probabilities[index] = values[index] / total
	}

	return probabilities, true
}

func (entropy *Entropy[T]) probabilityFloor(values []float64) float64 {
	if entropy.floor > 0 {
		return entropy.floor
	}

	total := floats.Sum(values)
	scale := total / float64(len(values))

	if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		return math.SmallestNonzeroFloat64
	}

	return math.Nextafter(0, scale)
}

type EntropyErrorType string

const (
	EntropyErrorNonFiniteMass EntropyErrorType = "sample mass is non-finite"
)

type EntropyError string

func (entropyError EntropyError) Error() string {
	return string(entropyError)
}

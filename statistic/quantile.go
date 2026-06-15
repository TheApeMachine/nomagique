package statistic

import (
	"math"
	"sort"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/core"
	"gonum.org/v1/gonum/stat"
)

/*
Quantile computes a sample quantile using gonum's stat.Quantile interpolation.
LinInterp matches linear interpolation between order statistics; Empirical uses
the step-function empirical distribution.
*/
type Quantile[T ~float64] struct {
	percentile float64
	kind       stat.CumulantKind
	weights    []float64
	output     core.Scalar[T]
}

/*
NewQuantile creates a quantile stage at percentile in [0, 1].
*/
func NewQuantile[T ~float64](
	percentile float64,
	kind stat.CumulantKind,
	weights []float64,
) *Quantile[T] {
	return &Quantile[T]{
		percentile: percentile,
		kind:       kind,
		weights:    weights,
	}
}

/*
Observe sorts the input stream and returns the configured quantile.
*/
func (quantile *Quantile[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	values := sampleBatch[T](inputs...)

	if len(values) == 0 {
		return quantile.output
	}

	weights := quantile.weights

	if len(weights) != 0 {
		if len(weights) != len(values) {
			errnie.Err(
				errnie.Validation, "unable to compute quantile",
				QuantileError(QuantileErrorWeightLengthMismatch),
			)

			return quantile.output
		}

		sortedValues, sortedWeights, ok := sortWeightedSamples(values, weights)

		if !ok {
			quantile.output = core.Scalar[T](T(math.NaN()))

			return quantile.output
		}

		quantile.output = quantile.quantileOf(sortedValues, sortedWeights)

		return quantile.output
	}

	sort.Float64s(values)
	quantile.output = quantile.quantileOf(values, nil)

	return quantile.output
}

func (quantile *Quantile[T]) Reset() error {
	quantile.weights = nil
	quantile.output = core.Scalar[T](0)

	return nil
}

func (quantile *Quantile[T]) quantileOf(
	sorted []float64, weights []float64,
) core.Scalar[T] {
	if quantile.percentile <= 0 {
		return core.Scalar[T](T(sorted[0]))
	}

	if quantile.percentile >= 1 {
		return core.Scalar[T](T(sorted[len(sorted)-1]))
	}

	if weights == nil {
		for _, value := range sorted {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return core.Scalar[T](T(math.NaN()))
			}
		}

		return core.Scalar[T](T(
			stat.Quantile(quantile.percentile, quantile.kind, sorted, nil),
		))
	}

	return core.Scalar[T](T(
		stat.Quantile(quantile.percentile, quantile.kind, sorted, weights),
	))
}

func sortWeightedSamples(values, weights []float64) ([]float64, []float64, bool) {
	for index, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, nil, false
		}

		weight := weights[index]

		if math.IsNaN(weight) || math.IsInf(weight, 0) || weight < 0 {
			return nil, nil, false
		}
	}

	sortedValues := make([]float64, len(values))
	copy(sortedValues, values)
	sortedWeights := make([]float64, len(weights))
	copy(sortedWeights, weights)

	stat.SortWeighted(sortedValues, sortedWeights)

	return sortedValues, sortedWeights, true
}

type QuantileErrorType string

const (
	QuantileErrorWeightLengthMismatch QuantileErrorType = "require equal weight length"
)

type QuantileError string

func (quantileError QuantileError) Error() string {
	return string(quantileError)
}

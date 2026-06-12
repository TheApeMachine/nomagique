package statistic

import (
	"math"
	"sort"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/core"
	"gonum.org/v1/gonum/stat"
)

/*
Quantile computes a sample quantile using gonum's stat.Quantile interpolation.
LinInterp matches linear interpolation between order statistics; Empirical uses
the step-function empirical distribution.
*/
type Quantile struct {
	percentile float64
	kind       stat.CumulantKind
	weights    core.Numbers
}

/*
NewQuantile creates a quantile dynamic at percentile in [0, 1].
*/
func NewQuantile(
	percentile float64,
	kind stat.CumulantKind,
	weights core.Numbers,
) *Quantile {
	return &Quantile{
		percentile: percentile,
		kind:       kind,
		weights:    weights,
	}
}

/*
Observe sorts the input stream and returns the configured quantile.
*/
func (quantile *Quantile) Observe(inputs ...core.Number) core.Float64 {
	values := nomagique.Samples(core.Numbers(inputs))

	if len(values) == 0 {
		return 0
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	return quantile.ObserveSorted(sorted)
}

/*
ObserveSorted returns the configured quantile from an ascending slice.
*/
func (quantile *Quantile) ObserveSorted(sorted []float64) core.Float64 {
	if len(sorted) == 0 {
		return 0
	}

	if quantile.percentile <= 0 {
		return core.Float64(sorted[0])
	}

	if quantile.percentile >= 1 {
		return core.Float64(sorted[len(sorted)-1])
	}

	weights := quantile.weights.Float64()

	if len(weights) == 0 {
		weights = nil
	}

	if len(weights) != 0 && len(weights) != len(sorted) {
		errnie.Err(
			errnie.Validation, "unable to compute quantile",
			QuantileError(QuantileErrorWeightLengthMismatch),
		)

		return 0
	}

	if weights == nil {
		for _, value := range sorted {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return core.Float64(math.NaN())
			}
		}

		return core.Float64(
			stat.Quantile(quantile.percentile, quantile.kind, sorted, nil),
		)
	}

	sortedValues, sortedWeights, ok := sortWeightedSamples(sorted, weights)

	if !ok {
		return core.Float64(math.NaN())
	}

	return core.Float64(
		stat.Quantile(quantile.percentile, quantile.kind, sortedValues, sortedWeights),
	)
}

func (quantile *Quantile) Reset() error {
	quantile.weights = nil
	return nil
}

/*
Quartiles returns the lower and upper quartiles at 0.25 and 0.75.
*/
type Quartiles struct {
	kind    stat.CumulantKind
	weights core.Numbers
}

/*
NewQuartiles creates a quartile pair dynamic.
*/
func NewQuartiles(kind stat.CumulantKind, weights core.Numbers) *Quartiles {
	return &Quartiles{
		kind:    kind,
		weights: weights,
	}
}

/*
Observe returns the lower and upper quartiles of the input stream.
*/
func (quartiles *Quartiles) Observe(inputs ...core.Number) (lower core.Float64, upper core.Float64) {
	lowerQuantile := NewQuantile(0.25, quartiles.kind, quartiles.weights)
	upperQuantile := NewQuantile(0.75, quartiles.kind, quartiles.weights)

	return lowerQuantile.Observe(inputs...), upperQuantile.Observe(inputs...)
}

func sortWeightedSamples(values, weights []float64) ([]float64, []float64, bool) {
	pairs := make([]weightedSample, len(values))

	for index, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, nil, false
		}

		weight := weights[index]

		if math.IsNaN(weight) || math.IsInf(weight, 0) || weight < 0 {
			return nil, nil, false
		}

		pairs[index] = weightedSample{
			value:  value,
			weight: weight,
		}
	}

	sort.Slice(pairs, func(leftIndex, rightIndex int) bool {
		return pairs[leftIndex].value < pairs[rightIndex].value
	})

	sortedValues := make([]float64, len(pairs))
	sortedWeights := make([]float64, len(pairs))

	for index, pair := range pairs {
		sortedValues[index] = pair.value
		sortedWeights[index] = pair.weight
	}

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

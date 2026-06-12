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
Median computes the sample median of a stream of numbers.
Unweighted samples use the conventional order-statistic definition: the middle
value for odd lengths and the average of the two central values for even lengths.
Weighted samples delegate to gonum's empirical quantile at p=0.5.
*/
type Median struct {
	weights core.Numbers
}

/*
NewMedian creates a new median dynamic.
*/
func NewMedian(weights core.Numbers) *Median {
	return &Median{
		weights: weights,
	}
}

/*
Observe computes the median of a stream of numbers.
*/
func (median *Median) Observe(inputs ...core.Number) core.Float64 {
	values := nomagique.Samples(core.Numbers(inputs))

	if len(values) == 0 {
		return 0
	}

	weights := median.weights.Float64()

	if len(weights) == 0 {
		return core.Float64(medianOf(values))
	}

	if len(weights) != len(values) {
		errnie.Err(
			errnie.Validation, "unable to compute median",
			MedianError(MedianErrorWeightLengthMismatch),
		)

		return 0
	}

	return core.Float64(weightedMedian(values, weights))
}

func (median *Median) Reset() error {
	median.weights = nil
	return nil
}

func medianOf(values []float64) float64 {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	middle := len(sorted) / 2

	if len(sorted)%2 == 1 {
		return sorted[middle]
	}

	return (sorted[middle-1] + sorted[middle]) / 2
}

func weightedMedian(values, weights []float64) float64 {
	sortedValues, sortedWeights, ok := sortWeightedSamples(values, weights)

	if !ok {
		return math.NaN()
	}

	return stat.Quantile(0.5, stat.Empirical, sortedValues, sortedWeights)
}

type weightedSample struct {
	value  float64
	weight float64
}

type MedianErrorType string

const (
	MedianErrorWeightLengthMismatch MedianErrorType = "require equal weight length"
)

type MedianError string

func (medianError MedianError) Error() string {
	return string(medianError)
}

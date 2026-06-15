package statistic

import (
	"math"
	"sort"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/core"
	"gonum.org/v1/gonum/stat"
)

/*
Median computes the sample median of a stream of numbers.
Unweighted samples use the conventional order-statistic definition: the middle
value for odd lengths and the average of the two central values for even lengths.
Weighted samples delegate to gonum's empirical quantile at p=0.5.
*/
type Median[T ~float64] struct {
	weights []float64
	output  core.Scalar[T]
}

/*
NewMedian creates a median stage.
*/
func NewMedian[T ~float64](weights []float64) *Median[T] {
	return &Median[T]{
		weights: weights,
	}
}

/*
Observe computes the median of a stream of numbers.
*/
func (median *Median[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	values := sampleBatch[T](inputs...)

	if len(values) == 0 {
		return median.output
	}

	weights := median.weights

	if len(weights) == 0 {
		median.output = core.Scalar[T](T(medianOf(values)))

		return median.output
	}

	if len(weights) != len(values) {
		errnie.Err(
			errnie.Validation, "unable to compute median",
			MedianError(MedianErrorWeightLengthMismatch),
		)

		return median.output
	}

	median.output = core.Scalar[T](T(weightedMedian(values, weights)))

	return median.output
}

func (median *Median[T]) Reset() error {
	median.weights = nil
	median.output = core.Scalar[T](0)

	return nil
}

/*
MedianOf returns the median of values without weights.
*/
func MedianOf(values []float64) float64 {
	return medianOf(values)
}

func medianOf(values []float64) float64 {
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return math.NaN()
		}
	}

	sort.Float64s(values)

	middle := len(values) / 2

	if len(values)%2 == 1 {
		return values[middle]
	}

	return (values[middle-1] + values[middle]) / 2
}

func weightedMedian(values, weights []float64) float64 {
	sortedValues, sortedWeights, ok := sortWeightedSamples(values, weights)

	if !ok {
		return math.NaN()
	}

	return stat.Quantile(0.5, stat.Empirical, sortedValues, sortedWeights)
}

type MedianErrorType string

const (
	MedianErrorWeightLengthMismatch MedianErrorType = "require equal weight length"
)

type MedianError string

func (medianError MedianError) Error() string {
	return string(medianError)
}

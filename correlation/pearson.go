package correlation

import (
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/core"
	"gonum.org/v1/gonum/stat"
)

/*
Pearson computes the Pearson correlation coefficient between two streams.
Optionally, weights can be provided which are applied to the inputs before
computing the correlation. This helps to reduce the impact of outliers.
*/
type Pearson[T ~float64] struct {
	weights []float64
	output  core.Scalar[T]
}

/*
NewPearson creates a new Pearson correlation dynamic.
*/
func NewPearson[T ~float64](weights []float64) *Pearson[T] {
	return &Pearson[T]{
		weights: weights,
	}
}

/*
Observe computes the Pearson correlation coefficient between two streams.
*/
func (pearson *Pearson[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	count := len(inputs)

	if count < 2 {
		errnie.Err(
			errnie.Validation, "unable to compute Pearson correlation",
			PearsonError(PearsonErrorRequireAtLeastTwoInputs),
		)

		return pearson.output
	}

	if count%2 != 0 {
		errnie.Err(
			errnie.Validation, "unable to compute Pearson correlation",
			PearsonError(PearsonErrorRequireEqualLength),
		)

		return pearson.output
	}

	half := count / 2
	left := sampleBatch[T](inputs[:half]...)
	right := sampleBatch[T](inputs[half:]...)

	pearson.output = core.Scalar[T](T(stat.Correlation(
		left, right, weightSamples(pearson.weights),
	)))

	return pearson.output
}

/*
Reset clears derived state.
*/
func (pearson *Pearson[T]) Reset() error {
	pearson.weights = nil
	pearson.output = core.Scalar[T](0)

	return nil
}

type PearsonErrorType string

const (
	PearsonErrorRequireAtLeastTwoInputs PearsonErrorType = "require at least two inputs"
	PearsonErrorRequireEqualLength      PearsonErrorType = "require equal length"
)

type PearsonError string

func (pearsonError PearsonError) Error() string {
	return string(pearsonError)
}

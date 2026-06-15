package correlation

import (
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/core"
	"gonum.org/v1/gonum/stat"
)

/*
Covariance computes sample covariance between two configured streams.
*/
type Covariance[T ~float64] struct {
	weights []float64
	output  core.Scalar[T]
}

/*
NewCovariance creates a covariance dynamic.
*/
func NewCovariance[T ~float64](weights []float64) *Covariance[T] {
	return &Covariance[T]{
		weights: weights,
	}
}

/*
Observe computes covariance between equal halves of the input stream.
*/
func (covariance *Covariance[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	count := len(inputs)

	if count < 2 {
		errnie.Err(
			errnie.Validation, "unable to compute covariance",
			CovarianceError(CovarianceErrorRequireAtLeastTwoInputs),
		)

		return covariance.output
	}

	if count%2 != 0 {
		errnie.Err(
			errnie.Validation, "unable to compute covariance",
			CovarianceError(CovarianceErrorRequireEqualLength),
		)

		return covariance.output
	}

	half := count / 2
	left := sampleBatch[T](inputs[:half]...)
	right := sampleBatch[T](inputs[half:]...)

	covariance.output = core.Scalar[T](T(stat.Covariance(
		left, right, weightSamples(covariance.weights),
	)))

	return covariance.output
}

/*
Reset clears derived state.
*/
func (covariance *Covariance[T]) Reset() error {
	covariance.weights = nil
	covariance.output = core.Scalar[T](0)

	return nil
}

type CovarianceErrorType string

const (
	CovarianceErrorRequireAtLeastTwoInputs CovarianceErrorType = "require at least two inputs"
	CovarianceErrorRequireEqualLength      CovarianceErrorType = "require equal length"
)

type CovarianceError string

func (covarianceError CovarianceError) Error() string {
	return string(covarianceError)
}

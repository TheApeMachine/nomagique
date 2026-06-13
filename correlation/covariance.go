package correlation

import (
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/core"
	"gonum.org/v1/gonum/stat"
)

/*
Covariance computes sample covariance between two configured streams.
*/
type Covariance struct {
	weights core.Numbers
}

/*
NewCovariance creates a covariance dynamic.
*/
func NewCovariance(weights core.Numbers) *Covariance {
	return &Covariance{
		weights: weights,
	}
}

/*
Observe computes covariance between equal halves of the input stream.
*/
func (covariance *Covariance) Observe(inputs ...core.Number) core.Float64 {
	count := len(inputs)

	if count < 2 {
		errnie.Err(
			errnie.Validation, "unable to compute covariance",
			CovarianceError(CovarianceErrorRequireAtLeastTwoInputs),
		)

		return 0
	}

	if count%2 != 0 {
		errnie.Err(
			errnie.Validation, "unable to compute covariance",
			CovarianceError(CovarianceErrorRequireEqualLength),
		)

		return 0
	}

	half := count / 2
	left := nomagique.Samples(core.Numbers(inputs[:half]))
	right := nomagique.Samples(core.Numbers(inputs[half:]))
	weights := nomagique.Samples(covariance.weights)

	if len(weights) == 0 {
		weights = nil
	}

	return core.Float64(stat.Covariance(left, right, weights))
}

/*
Reset clears derived state.
*/
func (covariance *Covariance) Reset() error {
	covariance.weights = nil
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

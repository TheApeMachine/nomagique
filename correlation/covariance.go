package correlation

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
Covariance computes sample covariance between two configured streams.
*/
type Covariance struct {
	artifact *datura.Artifact
	weights  []float64
}

/*
NewCovariance creates a covariance dynamic.
*/
func NewCovariance(weights []float64) *Covariance {
	return &Covariance{
		artifact: datura.Acquire("covariance", datura.Artifact_Type_json),
		weights:  weights,
	}
}

func (covariance *Covariance) Write(p []byte) (int, error) {
	return covariance.artifact.Write(p)
}

func (covariance *Covariance) Read(p []byte) (int, error) {
	values := float64Batch(covariance.artifact)
	count := len(values)

	if count >= 2 && count%2 == 0 {
		half := count / 2
		left := values[:half]
		right := values[half:]
		weights, weightsOK := weightSamplesFor(covariance.weights, half)

		if !weightsOK {
			putFloat64Payload(&covariance.artifact, "covariance", 0)

			return covariance.artifact.Read(p)
		}

		covarianceValue := stat.Covariance(left, right, weights)

		if math.IsNaN(covarianceValue) || math.IsInf(covarianceValue, 0) {
			covarianceValue = 0
		}

		putFloat64Payload(&covariance.artifact, "covariance", covarianceValue)
	}

	if count > 0 && count < 2 {
		errnie.Err(
			errnie.Validation, "unable to compute covariance",
			CovarianceError(CovarianceErrorRequireAtLeastTwoInputs),
		)
	}

	if count%2 != 0 && count > 0 {
		errnie.Err(
			errnie.Validation, "unable to compute covariance",
			CovarianceError(CovarianceErrorRequireEqualLength),
		)
	}

	return covariance.artifact.Read(p)
}

func (covariance *Covariance) Close() error {
	return nil
}

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

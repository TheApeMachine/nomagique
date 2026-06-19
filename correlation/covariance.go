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
		artifact: datura.Acquire("covariance", datura.APPJSON).RetainStageAttributes(),
		weights:  weights,
	}
}

func (covariance *Covariance) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](covariance.artifact, "output") == nil

	covariance.artifact.Clear("sample")
	covariance.artifact.Clear("paired")
	covariance.artifact.Clear("batch")
	covariance.artifact.Clear("left")
	covariance.artifact.Clear("right")

	n, err := covariance.artifact.Write(p)

	if bootstrap {
		covariance.artifact.Clear("output")
	}

	return n, err
}

func (covariance *Covariance) Read(p []byte) (int, error) {
	values := datura.Peek[[]float64](covariance.artifact, "batch")

	if len(values) == 0 {
		left := datura.Peek[[]float64](covariance.artifact, "left")
		right := datura.Peek[[]float64](covariance.artifact, "right")

		if len(left) > 0 || len(right) > 0 {
			values = append(append([]float64(nil), left...), right...)
		}
	}

	count := len(values)

	if count >= 2 && count%2 == 0 {
		half := count / 2
		left := values[:half]
		right := values[half:]

		weightsOK := len(covariance.weights) == 0 || len(covariance.weights) == half
		weights := covariance.weights

		for _, weight := range weights {
			if math.IsNaN(weight) || math.IsInf(weight, 0) || weight < 0 {
				weightsOK = false
			}
		}

		if len(weights) == 0 {
			weights = nil
		}

		if !weightsOK {
			covariance.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

			return covariance.artifact.Read(p)
		}

		covarianceValue := stat.Covariance(left, right, weights)

		if math.IsNaN(covarianceValue) || math.IsInf(covarianceValue, 0) {
			covarianceValue = 0
		}

		covariance.artifact.Poke(datura.Map[float64]{"value": covarianceValue}, "output")

		return covariance.artifact.Read(p)
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

	if count == 0 || count < 2 || count%2 != 0 {
		covariance.artifact.Poke(datura.Map[float64]{"value": 0}, "output")
	}

	return covariance.artifact.Read(p)
}

func (covariance *Covariance) Close() error {
	return nil
}

func (covariance *Covariance) Reset() error {
	covariance.weights = nil
	covariance.artifact.Clear("output")

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

package correlation

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
Covariance computes sample covariance between two configured streams.
Weights may be supplied on config.weights.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Covariance struct {
	artifact *datura.Artifact
}

/*
NewCovariance creates a covariance stage wired from config attributes on the artifact.
*/
func NewCovariance(artifact *datura.Artifact) *Covariance {
	return &Covariance{
		artifact: artifact,
	}
}

func (covariance *Covariance) Read(p []byte) (int, error) {
	state := datura.Acquire("covariance-state", datura.APPJSON)

	if _, err := state.Write(covariance.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"correlation-covariance: state write failed",
			err,
		))
	}


	values := datura.Peek[[]float64](state, "batch")

	if len(values) == 0 {
		left := datura.Peek[[]float64](state, "left")
		right := datura.Peek[[]float64](state, "right")

		if len(left) > 0 || len(right) > 0 {
			values = append(append([]float64(nil), left...), right...)
		}
	}

	count := len(values)

	if count >= 2 && count%2 == 0 {
		half := count / 2
		left := values[:half]
		right := values[half:]
		weights := datura.Peek[[]float64](covariance.artifact, "config", "weights")
		weightsOK := len(weights) == 0 || len(weights) == half

		for _, weight := range weights {
			if math.IsNaN(weight) || math.IsInf(weight, 0) || weight < 0 {
				weightsOK = false
			}
		}

		if len(weights) == 0 {
			weights = nil
		}

		if !weightsOK {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation, "unable to compute covariance",
				CovarianceError(CovarianceErrorInvalidWeights),
			))
		}

		covarianceValue := stat.Covariance(left, right, weights)

		if math.IsNaN(covarianceValue) || math.IsInf(covarianceValue, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation, "unable to compute covariance",
				CovarianceError(CovarianceErrorNonFiniteResult),
			))
		}

		state.MergeOutput("value", covarianceValue)
		state.Poke("output", "root")
		state.Poke([]string{"value"}, "inputs")
		return state.Read(p)
	}

	if count > 0 && count < 2 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "unable to compute covariance",
			CovarianceError(CovarianceErrorRequireAtLeastTwoInputs),
		))
	}

	if count%2 != 0 && count > 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "unable to compute covariance",
			CovarianceError(CovarianceErrorRequireEqualLength),
		))
	}

	return 0, errnie.Error(errnie.Err(
		errnie.Validation, "unable to compute covariance",
		CovarianceError(CovarianceErrorRequireAtLeastTwoInputs),
	))
}

func (covariance *Covariance) Write(p []byte) (int, error) {
	covariance.artifact.WithPayload(p)
	return len(p), nil
}

func (covariance *Covariance) Close() error {
	return nil
}

type CovarianceErrorType string

const (
	CovarianceErrorRequireAtLeastTwoInputs CovarianceErrorType = "require at least two inputs"
	CovarianceErrorRequireEqualLength      CovarianceErrorType = "require equal length"
	CovarianceErrorInvalidWeights          CovarianceErrorType = "require valid weights"
	CovarianceErrorNonFiniteResult         CovarianceErrorType = "require finite covariance"
)

type CovarianceError string

func (covarianceError CovarianceError) Error() string {
	return string(covarianceError)
}

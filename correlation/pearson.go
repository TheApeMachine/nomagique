package correlation

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
Pearson computes the Pearson correlation coefficient between two streams.
Weights may be supplied on config.weights and are applied before correlation.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Pearson struct {
	artifact *datura.Artifact
}

/*
NewPearson creates a Pearson correlation stage wired from config attributes on the artifact.
*/
func NewPearson(artifact *datura.Artifact) *Pearson {
	artifact.Inspect("correlation", "pearson", "NewPearson()")

	return &Pearson{
		artifact: artifact,
	}
}

func (pearson *Pearson) Write(p []byte) (int, error) {
	pearson.artifact.WithPayload(p)
	return len(p), nil
}

func (pearson *Pearson) Read(p []byte) (int, error) {
	state := datura.Acquire("pearson-state", datura.APPJSON)
	state.Inspect("correlation", "pearson", "Read()", "p")

	if _, err := state.Write(pearson.artifact.DecryptPayload()); err != nil {
		return 0, err
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
		weights := datura.Peek[[]float64](pearson.artifact, "config", "weights")
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
				errnie.Validation, "unable to compute Pearson correlation",
				PearsonError(PearsonErrorInvalidWeights),
			))
		}

		correlation := stat.Correlation(left, right, weights)

		if math.IsNaN(correlation) || math.IsInf(correlation, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation, "unable to compute Pearson correlation",
				PearsonError(PearsonErrorNonFiniteResult),
			))
		}

		state.MergeOutput("value", correlation)
		state.Merge("root", "output")
		state.Merge("inputs", []string{"value"})
		return state.Read(p)
	}

	if count > 0 && count < 2 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "unable to compute Pearson correlation",
			PearsonError(PearsonErrorRequireAtLeastTwoInputs),
		))
	}

	if count%2 != 0 && count > 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "unable to compute Pearson correlation",
			PearsonError(PearsonErrorRequireEqualLength),
		))
	}

	return 0, errnie.Error(errnie.Err(
		errnie.Validation, "unable to compute Pearson correlation",
		PearsonError(PearsonErrorRequireAtLeastTwoInputs),
	))
}

func (pearson *Pearson) Close() error {
	return nil
}

type PearsonErrorType string

const (
	PearsonErrorRequireAtLeastTwoInputs PearsonErrorType = "require at least two inputs"
	PearsonErrorRequireEqualLength      PearsonErrorType = "require equal length"
	PearsonErrorInvalidWeights          PearsonErrorType = "require valid weights"
	PearsonErrorNonFiniteResult         PearsonErrorType = "require finite correlation"
)

type PearsonError string

func (pearsonError PearsonError) Error() string {
	return string(pearsonError)
}

func inboundReset(payload []byte) bool {
	if len(payload) == 0 {
		return false
	}

	state := datura.Acquire("inbound-reset", datura.APPJSON)

	if _, err := state.Write(payload); err != nil {
		return false
	}

	return datura.Peek[float64](state, "reset") > 0
}

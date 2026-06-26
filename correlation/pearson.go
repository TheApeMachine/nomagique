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
	return &Pearson{
		artifact: artifact,
	}
}

func (pearson *Pearson) Read(p []byte) (int, error) {
	state := datura.Acquire("pearson-state", datura.APPJSON)

	if _, err := state.Unpack(pearson.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"correlation-pearson: state write failed",
			err,
		))
	}

	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"correlation-pearson: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"correlation-pearson: inputs required",
			nil,
		))
	}

	batchKey := datura.Peek[string](pearson.artifact, "input")

	if batchKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"correlation-pearson: input required",
			nil,
		))
	}

	var values []float64
	found := false

	for _, input := range inputs {
		if input != batchKey {
			continue
		}

		values = datura.Peek[[]float64](state, rootKey, input)
		found = true
	}

	if !found {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"correlation-pearson: input not in inputs",
			nil,
		))
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
		state.Poke("output", "root")
		state.Poke([]string{"value"}, "inputs")

		return state.PackInto(p)
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

func (pearson *Pearson) Write(p []byte) (int, error) {
	pearson.artifact.WithPayload(p)
	return len(p), nil
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

	if _, err := state.Unpack(payload); err != nil {
		return false
	}

	return datura.Peek[float64](state, "reset") > 0
}

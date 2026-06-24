package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
StdDev computes the sample standard deviation over retained history.
*/
type StdDev struct {
	artifact *datura.Artifact
}

/*
NewStdDev returns a standard-deviation stage wired from config attributes on the artifact.
*/
func NewStdDev(artifact *datura.Artifact) *StdDev {
	return &StdDev{
		artifact: artifact,
	}
}

func (stdDev *StdDev) Read(payload []byte) (int, error) {
	state := datura.Acquire("stddev-state", datura.APPJSON)

	if _, err := state.Write(stdDev.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"stddev: state write failed",
			err,
		))
	}

	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"stddev: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"stddev: inputs required",
			nil,
		))
	}

	configInput := datura.Peek[string](stdDev.artifact, "input")

	if configInput == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"stddev: input required",
			nil,
		))
	}

	outputKey := datura.Peek[string](stdDev.artifact, "outputKey")

	if outputKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"stddev: outputKey required",
			nil,
		))
	}

	var sample float64
	found := false

	for index, input := range inputs {
		if input != configInput {
			continue
		}

		if rootKey == "features" {
			features := datura.Peek[[]float64](state, rootKey)

			if index >= len(features) {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"stddev: feature index out of range",
					nil,
				))
			}

			sample = features[index]
		}

		if rootKey != "features" {
			sample = datura.Peek[float64](state, rootKey, input)
		}

		found = true
	}

	if !found {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"stddev: input not in inputs",
			nil,
		))
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"stddev: sample is non-finite",
			nil,
		))
	}

	history := datura.Peek[[]float64](stdDev.artifact, "history")
	history = append(history, sample)
	stdDev.artifact.Poke(history, "history")

	if len(history) < 2 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"stddev: at least two samples required",
			nil,
		))
	}

	value := stat.StdDev(history, nil)

	state.MergeOutput(outputKey, value)
	state.Poke("output", "root")
	state.Poke([]string{outputKey}, "inputs")

	return state.Read(payload)
}

func (stdDev *StdDev) Write(payload []byte) (int, error) {
	stdDev.artifact.WithPayload(payload)
	return len(payload), nil
}

func (stdDev *StdDev) Close() error {
	return nil
}

package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Sum integrates streamed samples into a running total.
*/
type Sum struct {
	artifact *datura.Artifact
}

/*
NewSum returns a sum stage wired from config attributes on the artifact.
*/
func NewSum(artifact *datura.Artifact) *Sum {
	return &Sum{
		artifact: artifact,
	}
}

func (sum *Sum) Read(payload []byte) (int, error) {
	state := datura.Acquire("sum-state", datura.APPJSON)

	if _, err := state.Write(sum.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"sum: state write failed",
			err,
		))
	}

	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"sum: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"sum: inputs required",
			nil,
		))
	}

	configInput := datura.Peek[string](sum.artifact, "input")

	if configInput == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"sum: input required",
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
					"sum: feature index out of range",
					nil,
				))
			}

			sample = features[index]
		}

		if rootKey != "features" {
			sample = datura.Peek[float64](state, rootKey, 0, input)
		}

		found = true
	}

	if !found {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"sum: input not in inputs",
			nil,
		))
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"sum: sample is non-finite",
			nil,
		))
	}

	value := datura.Peek[float64](sum.artifact, "output", "value") + sample

	sum.artifact.Poke(value, "output", "value")
	state.MergeOutput("value", value)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")

	return state.Read(payload)
}

func (sum *Sum) Write(payload []byte) (int, error) {
	sum.artifact.WithPayload(payload)
	return len(payload), nil
}

func (sum *Sum) Close() error {
	return nil
}

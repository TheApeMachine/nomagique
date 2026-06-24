package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Min tracks the smallest streamed sample.
*/
type Min struct {
	artifact *datura.Artifact
}

/*
NewMin returns a min stage wired from config attributes on the artifact.
*/
func NewMin(artifact *datura.Artifact) *Min {
	return &Min{
		artifact: artifact,
	}
}

func (min *Min) Read(payload []byte) (int, error) {
	state := datura.Acquire("min-state", datura.APPJSON)

	if _, err := state.Write(min.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"min: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"min: inputs required",
			nil,
		))
	}

	configInput := datura.Peek[string](min.artifact, "input")

	if configInput == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"min: input required",
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
					"min: feature index out of range",
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
			"min: input not in inputs",
			nil,
		))
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"min: sample is non-finite",
			nil,
		))
	}

	count := datura.Peek[float64](min.artifact, "output", "count")
	value := datura.Peek[float64](min.artifact, "output", "value")

	if count == 0 {
		value = sample
	}

	if count > 0 && sample < value {
		value = sample
	}

	count++
	min.artifact.Poke(count, "output", "count")
	min.artifact.Poke(value, "output", "value")
	state.MergeOutput("value", value)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")

	return state.Read(payload)
}

func (min *Min) Write(payload []byte) (int, error) {
	min.artifact.WithPayload(payload)
	return len(payload), nil
}

func (min *Min) Close() error {
	return nil
}

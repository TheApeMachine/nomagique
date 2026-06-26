package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Max tracks the largest streamed sample.
*/
type Max struct {
	artifact *datura.Artifact
}

/*
NewMax returns a max stage wired from config attributes on the artifact.
*/
func NewMax(artifact *datura.Artifact) *Max {
	return &Max{
		artifact: artifact,
	}
}

func (max *Max) Read(payload []byte) (int, error) {
	state := datura.Acquire("max-state", datura.APPJSON)

	if _, err := state.Unpack(max.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"max: state write failed",
			err,
		))
	}

	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"max: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"max: inputs required",
			nil,
		))
	}

	configInput := datura.Peek[string](max.artifact, "input")

	if configInput == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"max: input required",
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
					"max: feature index out of range",
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
			"max: input not in inputs",
			nil,
		))
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"max: sample is non-finite",
			nil,
		))
	}

	count := datura.Peek[float64](max.artifact, "output", "count")
	value := datura.Peek[float64](max.artifact, "output", "value")

	if count == 0 {
		value = sample
	}

	if count > 0 && sample > value {
		value = sample
	}

	count++
	max.artifact.Poke(count, "output", "count")
	max.artifact.Poke(value, "output", "value")
	state.MergeOutput("value", value)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")

	return state.PackInto(payload)
}

func (max *Max) Write(payload []byte) (int, error) {
	max.artifact.WithPayload(payload)
	return len(payload), nil
}

func (max *Max) Close() error {
	return nil
}

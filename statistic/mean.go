package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Mean computes a running arithmetic mean of streamed samples.
*/
type Mean struct {
	artifact *datura.Artifact
}

/*
NewMean returns a mean stage wired from config attributes on the artifact.
*/
func NewMean(artifact *datura.Artifact) *Mean {
	return &Mean{
		artifact: artifact,
	}
}

func (mean *Mean) Read(payload []byte) (int, error) {
	state := datura.Acquire("mean-state", datura.APPJSON)

	if _, err := state.Unpack(mean.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"mean: state write failed",
			err,
		))
	}

	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"mean: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"mean: inputs required",
			nil,
		))
	}

	configInput := datura.Peek[string](mean.artifact, "input")

	if configInput == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"mean: input required",
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
					"mean: feature index out of range",
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
			"mean: input not in inputs",
			nil,
		))
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"mean: sample is non-finite",
			nil,
		))
	}

	count := datura.Peek[float64](mean.artifact, "output", "count")
	sum := datura.Peek[float64](mean.artifact, "output", "sum")

	count++
	sum += sample

	value := sum / count

	mean.artifact.Poke(count, "output", "count")
	mean.artifact.Poke(sum, "output", "sum")
	mean.artifact.Poke(value, "output", "value")
	state.MergeOutput("value", value)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")

	return state.PackInto(payload)
}

func (mean *Mean) Write(payload []byte) (int, error) {
	mean.artifact.WithPayload(payload)
	return len(payload), nil
}

func (mean *Mean) Close() error {
	return nil
}

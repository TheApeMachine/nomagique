package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Delta tracks a unit-normalized change relative to the running sample range.
The constructor artifact holds config; Write buffers inbound payload.
*/
type Delta struct {
	artifact *datura.Artifact
	min      float64
	max      float64
	prev     float64
}

/*
NewDelta returns a delta stage wired from config attributes on the artifact.
*/
func NewDelta(artifact *datura.Artifact) *Delta {
	return &Delta{
		artifact: artifact,
	}
}

func (delta *Delta) Read(payload []byte) (int, error) {
	state := datura.Acquire("delta-state", datura.APPJSON)

	if _, err := state.Write(delta.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"delta: state write failed",
			err,
		))
	}


	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"delta: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"delta: inputs required",
			nil,
		))
	}

	configInput := datura.Peek[string](delta.artifact, "input")

	if configInput == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"delta: input required",
			nil,
		))
	}

	var found bool

	for index, input := range inputs {
		if input != configInput {
			continue
		}

		found = true
		var sample float64

		if rootKey == "features" {
			features := datura.Peek[[]float64](state, rootKey)

			if index >= len(features) {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"delta: feature index out of range",
					nil,
				))
			}

			sample = features[index]
		}

		if rootKey != "features" {
			sample = datura.Peek[float64](state, rootKey, input)
		}

		if math.IsNaN(sample) || math.IsInf(sample, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"delta: sample is non-finite",
				nil,
			))
		}

		count := datura.Peek[float64](delta.artifact, "output", "count")

		if count == 0 {
			delta.min = sample
			delta.max = sample
			delta.prev = sample
			delta.artifact.Poke(1.0, "output", "count")
			state.MergeOutput("value", 0)

			break
		}

		delta.min = math.Min(delta.min, sample)
		delta.max = math.Max(delta.max, sample)

		span := delta.max - delta.min

		if span == 0 {
			delta.prev = sample

			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"delta: sample span is zero",
				nil,
			))
		}

		value := math.Abs(sample-delta.prev) / span
		delta.prev = sample
		delta.artifact.Poke(count+1, "output", "count")

		state.MergeOutput("value", value)
	}

	if !found {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"delta: input not in inputs",
			nil,
		))
	}

	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")

	return state.Read(payload)
}

func (delta *Delta) Write(p []byte) (int, error) {
	delta.artifact.WithPayload(p)
	return len(p), nil
}

func (delta *Delta) Close() error {
	return nil
}

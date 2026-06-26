package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Momentum tracks a signed unit-normalized move relative to the running range.
The constructor artifact holds config; Write buffers inbound payload.
*/
type Momentum struct {
	artifact *datura.Artifact
	min      float64
	max      float64
	prev     float64
}

/*
NewMomentum returns a momentum stage wired from config attributes on the artifact.
*/
func NewMomentum(artifact *datura.Artifact) *Momentum {
	return &Momentum{
		artifact: artifact,
	}
}

func (momentum *Momentum) Read(payload []byte) (int, error) {
	state := datura.Acquire("momentum-state", datura.APPJSON)

	if _, err := state.Unpack(momentum.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"momentum: state write failed",
			err,
		))
	}

	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"momentum: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"momentum: inputs required",
			nil,
		))
	}

	configInput := datura.Peek[string](momentum.artifact, "input")

	if configInput == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"momentum: input required",
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
					"momentum: feature index out of range",
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
				"momentum: sample is non-finite",
				nil,
			))
		}

		count := datura.Peek[float64](momentum.artifact, "output", "count")

		if count == 0 {
			momentum.min = sample
			momentum.max = sample
			momentum.prev = sample
			momentum.artifact.Poke(1.0, "output", "count")
			state.MergeOutput("value", 0)

			break
		}

		momentum.min = math.Min(momentum.min, sample)
		momentum.max = math.Max(momentum.max, sample)

		span := momentum.max - momentum.min

		if span == 0 {
			momentum.prev = sample

			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"momentum: sample span is zero",
				nil,
			))
		}

		value := (sample - momentum.prev) / span
		momentum.prev = sample
		momentum.artifact.Poke(count+1, "output", "count")

		state.MergeOutput("value", value)
	}

	if !found {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"momentum: input not in inputs",
			nil,
		))
	}

	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")

	return state.PackInto(payload)
}

func (momentum *Momentum) Write(p []byte) (int, error) {
	momentum.artifact.WithPayload(p)
	return len(p), nil
}

func (momentum *Momentum) Close() error {
	return nil
}

package geometry

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Velocity tracks mean velocity between consecutive observations.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Velocity struct {
	artifact *datura.Artifact
	prev     float64
	count    int
}

/*
NewVelocity returns a velocity stage wired from config attributes on the artifact.
*/
func NewVelocity(artifact *datura.Artifact) *Velocity {
	return &Velocity{
		artifact: artifact,
	}
}

func (velocity *Velocity) Read(payload []byte) (int, error) {
	state := datura.Acquire("velocity-state", datura.APPJSON)

	if _, err := state.Unpack(velocity.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"velocity: state write failed",
			err,
		))
	}

	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"velocity: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"velocity: inputs required",
			nil,
		))
	}

	configInput := datura.Peek[string](velocity.artifact, "input")

	if configInput == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"velocity: input required",
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
					"velocity: feature index out of range",
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
			"velocity: input not in inputs",
			nil,
		))
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"velocity: sample is non-finite",
			nil,
		))
	}

	derived := 0.0

	if velocity.count == 0 {
		velocity.prev = sample
		velocity.count = 1

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"velocity: sample span is zero",
			nil,
		))
	}

	derived = sample - velocity.prev
	velocity.prev = sample
	velocity.count++

	velocity.artifact.Poke(derived, "output", "value")
	state.MergeOutput("value", derived)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")

	return state.PackInto(payload)
}

func (velocity *Velocity) Write(payload []byte) (int, error) {
	velocity.artifact.WithPayload(payload)
	return len(payload), nil
}

func (velocity *Velocity) Close() error {
	return nil
}

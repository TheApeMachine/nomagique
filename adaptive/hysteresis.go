package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Hysteresis debounces a binary signal so brief trips do not flip state.
The constructor artifact holds config; Write buffers inbound payload.
*/
type Hysteresis struct {
	artifact    *datura.Artifact
	value       float64
	pendingHigh float64
	pendingLow  float64
}

/*
NewHysteresis returns a hysteresis stage wired from config attributes on the artifact.
*/
func NewHysteresis(artifact *datura.Artifact) *Hysteresis {
	return &Hysteresis{
		artifact: artifact,
	}
}

func (hysteresis *Hysteresis) Read(payload []byte) (int, error) {
	state := datura.Acquire("hysteresis-state", datura.APPJSON)

	if _, err := state.Unpack(hysteresis.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"hysteresis: state write failed",
			err,
		))
	}

	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"hysteresis: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"hysteresis: inputs required",
			nil,
		))
	}

	inputKey := datura.Peek[string](hysteresis.artifact, "input")

	if inputKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"hysteresis: input required",
			nil,
		))
	}

	var sample float64
	found := false

	for index, input := range inputs {
		if input != inputKey {
			continue
		}

		if rootKey == "features" {
			features := datura.Peek[[]float64](state, rootKey)

			if index >= len(features) {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"hysteresis: feature index out of range",
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
			"hysteresis: input not in inputs",
			nil,
		))
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"hysteresis: sample is non-finite",
			nil,
		))
	}

	window := int(datura.Peek[float64](hysteresis.artifact, "window"))

	if window <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"hysteresis: window required",
			nil,
		))
	}

	threshold := datura.Peek[float64](hysteresis.artifact, "threshold")
	isHigh := sample > threshold

	if isHigh {
		hysteresis.pendingHigh++
		hysteresis.pendingLow = 0

		if hysteresis.pendingHigh >= float64(window) {
			hysteresis.value = 1
		}
	}

	if !isHigh {
		hysteresis.pendingLow++
		hysteresis.pendingHigh = 0

		if hysteresis.pendingLow >= float64(window) {
			hysteresis.value = 0
		}
	}

	if math.IsNaN(hysteresis.value) || math.IsInf(hysteresis.value, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"hysteresis: output value is non-finite",
			nil,
		))
	}

	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")
	state.MergeOutput("value", hysteresis.value)

	return state.PackInto(payload)
}

func (hysteresis *Hysteresis) Write(p []byte) (int, error) {
	hysteresis.artifact.WithPlaintextPayload(p)
	return len(p), nil
}

func (hysteresis *Hysteresis) Close() error {
	return nil
}

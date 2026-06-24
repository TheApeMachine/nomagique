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

	if _, err := state.Write(hysteresis.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"hysteresis: state write failed",
			err,
		))
	}


	inputKey := datura.Peek[string](hysteresis.artifact, "input")

	if inputKey == "" {
		inputKey = "sample"
	}

	root := datura.Peek[string](state, "root")
	sample := datura.Peek[float64](state, root, inputKey)

	if root == "" {
		sample = datura.Peek[float64](state, inputKey)
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"hysteresis: sample is non-finite",
			nil,
		))
	}

	window := int(datura.Peek[float64](state, "window"))

	if window <= 0 {
		window = int(datura.Peek[float64](hysteresis.artifact, "window"))
	}

	if window <= 0 {
		history := int(datura.Peek[float64](hysteresis.artifact, "history"))

		if history > 0 {
			window = int(math.Ceil(math.Sqrt(float64(history))))

			if window < 1 {
				window = 1
			}
		}
	}

	if window <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"hysteresis: window must be positive",
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

	return state.Read(payload)
}

func (hysteresis *Hysteresis) Write(p []byte) (int, error) {
	hysteresis.artifact.WithPayload(p)
	return len(p), nil
}

func (hysteresis *Hysteresis) Close() error {
	return nil
}

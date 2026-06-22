package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/statistic"
)

/*
Hysteresis debounces a binary signal so brief trips do not flip state.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Hysteresis struct {
	artifact *datura.Artifact
}

/*
NewHysteresis returns a hysteresis stage wired from config attributes on the artifact.
*/
func NewHysteresis(artifact *datura.Artifact) *Hysteresis {
	artifact.Inspect("adaptive", "hysteresis", "NewHysteresis()")

	return &Hysteresis{
		artifact: artifact,
	}
}

func (hysteresis *Hysteresis) Write(payload []byte) (int, error) {
	hysteresis.artifact.WithPayload(payload)
	return len(payload), nil
}

func (hysteresis *Hysteresis) Read(payload []byte) (int, error) {
	state := datura.Acquire("hysteresis-state", datura.APPJSON)
	state.Inspect("adaptive", "hysteresis", "Read()", "p")

	if _, err := state.Write(hysteresis.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	features := statistic.SnapshotFeatures(state)
	sample := hysteresis.sample(state)

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
	value := datura.Peek[float64](hysteresis.artifact, "output", "value")
	pendingHigh := datura.Peek[float64](hysteresis.artifact, "output", "pendingHigh")
	pendingLow := datura.Peek[float64](hysteresis.artifact, "output", "pendingLow")

	if math.IsNaN(value) || math.IsInf(value, 0) {
		value = 0
	}

	isHigh := sample > threshold

	if isHigh {
		pendingHigh++
		pendingLow = 0

		if pendingHigh >= float64(window) {
			value = 1
		}
	}

	if !isHigh {
		pendingLow++
		pendingHigh = 0

		if pendingLow >= float64(window) {
			value = 0
		}
	}

	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"hysteresis: output value is non-finite",
			nil,
		))
	}

	hysteresis.artifact.Poke(value, "output", "value")
	hysteresis.artifact.Poke(pendingHigh, "output", "pendingHigh")
	hysteresis.artifact.Poke(pendingLow, "output", "pendingLow")
	state.MergeOutput("value", value)
	features.Restore(state)
	state.Merge("root", "output")

	if len(datura.Peek[[]string](state, "inputs")) == 0 {
		state.Merge("inputs", []string{"value"})
	}

	return state.Read(payload)
}

func (hysteresis *Hysteresis) sample(state *datura.Artifact) float64 {
	inputKey := datura.Peek[string](hysteresis.artifact, "input")

	if inputKey == "" {
		inputKey = "sample"
	}

	if datura.Peek[string](state, "root") == "output" && inputKey != "sample" {
		return datura.Peek[float64](state, "output", inputKey)
	}

	return datura.Peek[float64](state, inputKey)
}

func (hysteresis *Hysteresis) Close() error {
	return nil
}

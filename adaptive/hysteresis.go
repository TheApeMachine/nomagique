package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
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
	output := datura.Peek[datura.Map[float64]](hysteresis.artifact, "output")

	hysteresis.artifact.WithPayload(payload)

	if output != nil {
		hysteresis.artifact.Merge("output", output)
	}

	return len(payload), nil
}

func (hysteresis *Hysteresis) Read(payload []byte) (int, error) {
	state := datura.Acquire("hysteresis-state", datura.APPJSON)
	state.Inspect("adaptive", "hysteresis", "Read()", "p")

	if _, err := state.Write(hysteresis.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	sample := datura.Peek[float64](state, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return state.Read(payload)
	}

	window := int(datura.Peek[float64](state, "config", "window"))

	if window <= 0 {
		window = int(datura.Peek[float64](hysteresis.artifact, "config", "window"))
	}

	if window <= 0 {
		history := int(datura.Peek[float64](hysteresis.artifact, "config", "history"))

		if history <= 0 {
			window = 2
		}

		if history > 0 {
			window = int(math.Ceil(math.Sqrt(float64(history))))

			if window < 2 {
				window = 2
			}
		}
	}

	output := datura.Peek[datura.Map[float64]](hysteresis.artifact, "output")

	if output == nil {
		output = datura.Map[float64]{
			"value":       0,
			"pendingHigh": 0,
			"pendingLow":  0,
		}
	}

	if sample != 0 {
		output["pendingHigh"]++
		output["pendingLow"] = 0

		if output["pendingHigh"] >= float64(window) {
			output["value"] = 1
		}
	}

	if sample == 0 {
		output["pendingLow"]++
		output["pendingHigh"] = 0

		if output["pendingLow"] >= float64(window) {
			output["value"] = 0
		}
	}

	hysteresis.artifact.Merge("output", output)
	state.MergeOutput("value", output["value"])
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(payload)
}

func (hysteresis *Hysteresis) Close() error {
	return nil
}

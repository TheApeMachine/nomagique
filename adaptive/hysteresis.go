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
	hysteresis.artifact.WithPayload(payload)
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

	value := datura.Peek[float64](hysteresis.artifact, "output", "value")
	pendingHigh := datura.Peek[float64](hysteresis.artifact, "output", "pendingHigh")
	pendingLow := datura.Peek[float64](hysteresis.artifact, "output", "pendingLow")

	if sample != 0 {
		pendingHigh++
		pendingLow = 0

		if pendingHigh >= float64(window) {
			value = 1
		}
	}

	if sample == 0 {
		pendingLow++
		pendingHigh = 0

		if pendingLow >= float64(window) {
			value = 0
		}
	}

	hysteresis.artifact.Poke(value, "output", "value")
	hysteresis.artifact.Poke(pendingHigh, "output", "pendingHigh")
	hysteresis.artifact.Poke(pendingLow, "output", "pendingLow")
	state.MergeOutput("value", value)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(payload)
}

func (hysteresis *Hysteresis) Close() error {
	return nil
}

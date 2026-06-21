package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
Accumulator integrates signed signal strength into a level with no bounds.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Accumulator struct {
	artifact *datura.Artifact
}

/*
NewAccumulator returns an accumulator stage wired from config attributes on the artifact.
*/
func NewAccumulator(artifact *datura.Artifact) *Accumulator {
	artifact.Inspect("adaptive", "accumulator", "NewAccumulator()")

	return &Accumulator{
		artifact: artifact,
	}
}

func (accumulator *Accumulator) Write(payload []byte) (int, error) {
	accumulator.artifact.WithPayload(payload)
	return len(payload), nil
}

func (accumulator *Accumulator) Read(payload []byte) (int, error) {
	state := datura.Acquire("accumulator-state", datura.APPJSON)
	state.Inspect("adaptive", "accumulator", "Read()", "p")

	if _, err := state.Write(accumulator.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	sample := datura.Peek[float64](state, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return state.Read(payload)
	}

	output := datura.Peek[datura.Map[float64]](accumulator.artifact, "output")

	if output == nil {
		output = datura.Map[float64]{
			"value": 0,
		}
	}

	if sample != 0 {
		output["value"] += sample
	}

	accumulator.artifact.Poke(output, "output")
	state.MergeOutput("value", output["value"])
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(payload)
}

func (accumulator *Accumulator) Close() error {
	return nil
}

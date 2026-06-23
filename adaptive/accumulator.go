package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Accumulator integrates signed signal strength into a level with no bounds.
The constructor artifact holds config; Write buffers inbound wire on artifact.
*/
type Accumulator struct {
	artifact *datura.Artifact
	total    float64
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

func (accumulator *Accumulator) Write(p []byte) (int, error) {
	accumulator.artifact.WithPayload(p)
	return len(p), nil
}

func (accumulator *Accumulator) Read(payload []byte) (int, error) {
	state := datura.Acquire("accumulator-state", datura.APPJSON)
	state.Inspect("adaptive", "accumulator", "Read()", "p")

	if _, err := state.Write(accumulator.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"accumulator: state write failed",
			err,
		))
	}

	root := datura.Peek[string](state, "root")
	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		inputs = []string{"sample"}
	}

	for _, input := range inputs {
		sample := datura.Peek[float64](state, root, input)

		if root == "" {
			sample = datura.Peek[float64](state, input)
		}

		if math.IsNaN(sample) || math.IsInf(sample, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"accumulator: sample is non-finite",
				nil,
			))
		}

		accumulator.total += sample
	}

	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")
	state.MergeOutput("value", accumulator.total)

	return state.Read(payload)
}

func (accumulator *Accumulator) Close() error {
	return nil
}

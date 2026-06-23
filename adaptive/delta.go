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
	artifact     *datura.Artifact
	bootstrapped bool
	min          float64
	max          float64
	prev         float64
}

/*
NewDelta returns a delta stage wired from config attributes on the artifact.
*/
func NewDelta(artifact *datura.Artifact) *Delta {
	artifact.Inspect("adaptive", "delta", "NewDelta()")

	return &Delta{
		artifact: artifact,
	}
}

func (delta *Delta) Write(p []byte) (int, error) {
	delta.artifact.WithPayload(p)
	return len(p), nil
}

func (delta *Delta) Read(payload []byte) (int, error) {
	state := datura.Acquire("delta-state", datura.APPJSON)
	state.Inspect("adaptive", "delta", "Read()", "p")

	if _, err := state.Write(delta.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"delta: state write failed",
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
				"delta: sample is non-finite",
				nil,
			))
		}

		if !delta.bootstrapped {
			delta.min = sample
			delta.max = sample
			delta.prev = sample
			delta.bootstrapped = true

			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"delta: insufficient samples",
				nil,
			))
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

		state.Poke("output", "root")
		state.Poke([]string{"value"}, "inputs")
		state.MergeOutput("value", value)
	}

	return state.Read(payload)
}

func (delta *Delta) Close() error {
	return nil
}

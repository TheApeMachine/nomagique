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
	artifact     *datura.Artifact
	bootstrapped bool
	min          float64
	max          float64
	prev         float64
}

/*
NewMomentum returns a momentum stage wired from config attributes on the artifact.
*/
func NewMomentum(artifact *datura.Artifact) *Momentum {
	artifact.Inspect("adaptive", "momentum", "NewMomentum()")

	return &Momentum{
		artifact: artifact,
	}
}

func (momentum *Momentum) Write(p []byte) (int, error) {
	momentum.artifact.WithPayload(p)
	return len(p), nil
}

func (momentum *Momentum) Read(payload []byte) (int, error) {
	state := datura.Acquire("momentum-state", datura.APPJSON)
	state.Inspect("adaptive", "momentum", "Read()", "p")

	if _, err := state.Write(momentum.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"momentum: state write failed",
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
				"momentum: sample is non-finite",
				nil,
			))
		}

		if !momentum.bootstrapped {
			momentum.min = sample
			momentum.max = sample
			momentum.prev = sample
			momentum.bootstrapped = true

			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"momentum: insufficient samples",
				nil,
			))
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

		state.Poke("output", "root")
		state.Poke([]string{"value"}, "inputs")
		state.MergeOutput("value", value)
	}

	return state.Read(payload)
}

func (momentum *Momentum) Close() error {
	return nil
}

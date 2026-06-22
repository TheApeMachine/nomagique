package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Delta tracks a unit-normalized change relative to the running sample range.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Delta struct {
	artifact     *datura.Artifact
	bootstrapped bool
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

func (delta *Delta) Write(payload []byte) (int, error) {
	delta.artifact.WithPayload(payload)
	return len(payload), nil
}

func (delta *Delta) Read(payload []byte) (int, error) {
	state := datura.Acquire("delta-state", datura.APPJSON)
	state.Inspect("adaptive", "delta", "Read()", "p")

	if _, err := state.Write(delta.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	sample := datura.Peek[float64](state, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"delta: sample is non-finite",
			nil,
		))
	}

	output := datura.Map[float64]{
		"min":   datura.Peek[float64](delta.artifact, "output", "min"),
		"max":   datura.Peek[float64](delta.artifact, "output", "max"),
		"prev":  datura.Peek[float64](delta.artifact, "output", "prev"),
		"value": datura.Peek[float64](delta.artifact, "output", "value"),
	}

	if !delta.bootstrapped {
		output = datura.Map[float64]{
			"min":   sample,
			"max":   sample,
			"prev":  sample,
			"value": 0,
		}

		delta.bootstrapped = true
		delta.artifact.Poke(output, "output")

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"delta: insufficient samples",
			nil,
		))
	}

	output["min"] = math.Min(output["min"], sample)
	output["max"] = math.Max(output["max"], sample)

	span := output["max"] - output["min"]

	if span == 0 {
		output["prev"] = sample
		delta.artifact.Poke(output, "output")

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"delta: sample span is zero",
			nil,
		))
	}

	output["value"] = math.Abs(sample-output["prev"]) / span
	output["prev"] = sample

	delta.artifact.Poke(output, "output")
	state.MergeOutput("value", output["value"])
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(payload)
}

func (delta *Delta) Close() error {
	return nil
}

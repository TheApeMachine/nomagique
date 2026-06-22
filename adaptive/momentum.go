package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/statistic"
)

/*
Momentum tracks a signed unit-normalized move relative to the running range.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Momentum struct {
	artifact     *datura.Artifact
	bootstrapped bool
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

func (momentum *Momentum) Write(payload []byte) (int, error) {
	momentum.artifact.WithPayload(payload)
	return len(payload), nil
}

func (momentum *Momentum) Read(payload []byte) (int, error) {
	state := datura.Acquire("momentum-state", datura.APPJSON)
	state.Inspect("adaptive", "momentum", "Read()", "p")

	if _, err := state.Write(momentum.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	features := statistic.SnapshotFeatures(state)
	sample := datura.Peek[float64](state, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"momentum: sample is non-finite",
			nil,
		))
	}

	output := datura.Map[float64]{
		"min":   datura.Peek[float64](momentum.artifact, "output", "min"),
		"max":   datura.Peek[float64](momentum.artifact, "output", "max"),
		"prev":  datura.Peek[float64](momentum.artifact, "output", "prev"),
		"value": datura.Peek[float64](momentum.artifact, "output", "value"),
	}

	if !momentum.bootstrapped {
		output = datura.Map[float64]{
			"min":   sample,
			"max":   sample,
			"prev":  sample,
			"value": 0,
		}

		momentum.bootstrapped = true
		momentum.artifact.Poke(output, "output")
		state.MergeOutput("value", output["value"])
		features.Restore(state)
		state.Merge("root", "output")

		if len(datura.Peek[[]string](state, "inputs")) == 0 {
			state.Merge("inputs", []string{"value"})
		}

		return state.Read(payload)
	}

	output["min"] = math.Min(output["min"], sample)
	output["max"] = math.Max(output["max"], sample)

	span := output["max"] - output["min"]

	if span == 0 {
		output["prev"] = sample
		momentum.artifact.Poke(output, "output")

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"momentum: sample span is zero",
			nil,
		))
	}

	output["value"] = (sample - output["prev"]) / span
	output["prev"] = sample

	momentum.artifact.Poke(output, "output")
	state.MergeOutput("value", output["value"])
	features.Restore(state)
	state.Merge("root", "output")

	if len(datura.Peek[[]string](state, "inputs")) == 0 {
		state.Merge("inputs", []string{"value"})
	}

	return state.Read(payload)
}

func (momentum *Momentum) Close() error {
	return nil
}

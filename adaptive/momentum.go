package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Momentum tracks a signed unit-normalized move relative to the running range.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Momentum struct {
	artifact *datura.Artifact
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

	sample := datura.Peek[float64](state, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"momentum: sample is non-finite",
			nil,
		))
	}

	output := datura.Peek[datura.Map[float64]](momentum.artifact, "output")

	if output == nil {
		output = datura.Map[float64]{
			"min":   sample,
			"max":   sample,
			"prev":  sample,
			"value": 0,
		}

		momentum.artifact.Poke(output, "output")
		state.MergeOutput("value", output["value"])
		state.Merge("root", "output")
		state.Merge("inputs", []string{"value"})
		return state.Read(payload)
	}

	output["min"] = math.Min(output["min"], sample)
	output["max"] = math.Max(output["max"], sample)

	span := output["max"] - output["min"]

	if span == 0 {
		output["prev"] = sample
		momentum.artifact.Poke(output, "output")
		state.MergeOutput("value", output["value"])
		state.Merge("root", "output")
		state.Merge("inputs", []string{"value"})
		return state.Read(payload)
	}

	output["value"] = (sample - output["prev"]) / span
	output["prev"] = sample

	momentum.artifact.Poke(output, "output")
	state.MergeOutput("value", output["value"])
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(payload)
}

func (momentum *Momentum) Close() error {
	return nil
}

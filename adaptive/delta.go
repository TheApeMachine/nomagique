package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
Delta tracks a unit-normalized change relative to the running sample range.
*/
type Delta struct {
	artifact *datura.Artifact
}

/*
NewDelta returns a delta stage ready to bootstrap from its first observation.
*/
func NewDelta() *Delta {
	return &Delta{
		artifact: datura.Acquire("delta", datura.APPJSON),
	}
}

func (delta *Delta) Read(p []byte) (int, error) {
	sample := datura.Peek[float64](delta.artifact, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return delta.artifact.Read(p)
	}

	output := datura.Peek[datura.Map[float64]](delta.artifact, "output")

	if output == nil {
		output = datura.Map[float64]{
			"min":   sample,
			"max":   sample,
			"prev":  sample,
			"value": 0,
		}

		delta.artifact.Poke(output, "output")

		return delta.artifact.Read(p)
	}

	output["min"] = math.Min(output["min"], sample)
	output["max"] = math.Max(output["max"], sample)

	span := output["max"] - output["min"]

	if span == 0 {
		output["prev"] = sample
		delta.artifact.Poke(output, "output")

		return delta.artifact.Read(p)
	}

	output["value"] = math.Abs(sample-output["prev"]) / span
	output["prev"] = sample

	delta.artifact.Poke(output, "output")

	return delta.artifact.Read(p)
}

func (delta *Delta) Write(p []byte) (int, error) {
	return delta.artifact.Write(p)
}

func (delta *Delta) Close() error {
	return nil
}

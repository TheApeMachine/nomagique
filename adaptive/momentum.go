package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
Momentum tracks a signed unit-normalized move relative to the running range.
*/
type Momentum struct {
	artifact *datura.Artifact
}

/*
NewMomentum returns a momentum stage ready to bootstrap from its first observation.
*/
func NewMomentum() *Momentum {
	return &Momentum{
		artifact: datura.Acquire("momentum", datura.APPJSON).RetainStageAttributes(),
	}
}

func (momentum *Momentum) Read(p []byte) (int, error) {
	sample := datura.Peek[float64](momentum.artifact, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return momentum.artifact.Read(p)
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

		return momentum.artifact.Read(p)
	}

	output["min"] = math.Min(output["min"], sample)
	output["max"] = math.Max(output["max"], sample)

	span := output["max"] - output["min"]

	if span == 0 {
		output["prev"] = sample
		momentum.artifact.Poke(output, "output")

		return momentum.artifact.Read(p)
	}

	output["value"] = (sample - output["prev"]) / span
	output["prev"] = sample

	momentum.artifact.Poke(output, "output")

	return momentum.artifact.Read(p)
}

func (momentum *Momentum) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](momentum.artifact, "output") == nil

	momentum.artifact.Clear("sample")

	n, err := momentum.artifact.Write(p)

	if bootstrap {
		momentum.artifact.Clear("output")
	}

	return n, err
}

func (momentum *Momentum) Close() error {
	return nil
}

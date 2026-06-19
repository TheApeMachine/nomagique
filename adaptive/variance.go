package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
Variance tracks an adaptive mean and variance from the observed sample stream.
*/
type Variance struct {
	artifact *datura.Artifact
}

/*
NewVariance returns a variance stage ready to bootstrap from its first observation.
*/
func NewVariance() *Variance {
	return &Variance{
		artifact: datura.Acquire("variance", datura.APPJSON).RetainStageAttributes(),
	}
}

func (variance *Variance) Read(p []byte) (int, error) {
	sample := datura.Peek[float64](variance.artifact, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return variance.artifact.Read(p)
	}

	output := datura.Peek[datura.Map[float64]](variance.artifact, "output")

	if output == nil {
		output = datura.Map[float64]{
			"mean":  sample,
			"var":   0,
			"prev":  sample,
			"min":   sample,
			"max":   sample,
			"rate":  0,
			"value": 0,
		}

		variance.artifact.Poke(output, "output")

		return variance.artifact.Read(p)
	}

	output["min"] = math.Min(output["min"], sample)
	output["max"] = math.Max(output["max"], sample)

	span := output["max"] - output["min"]

	if span == 0 {
		output["prev"] = sample
		variance.artifact.Poke(output, "output")

		return variance.artifact.Read(p)
	}

	delta := math.Abs(sample - output["prev"])
	output["rate"] = delta / span
	deviation := sample - output["mean"]
	output["mean"] += output["rate"] * (sample - output["mean"])
	output["var"] += output["rate"] * (deviation*deviation - output["var"])
	output["prev"] = sample
	output["value"] = output["var"]

	variance.artifact.Poke(output, "output")

	return variance.artifact.Read(p)
}

func (variance *Variance) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](variance.artifact, "output") == nil

	variance.artifact.Clear("sample")

	n, err := variance.artifact.Write(p)

	if bootstrap {
		variance.artifact.Clear("output")
	}

	return n, err
}

func (variance *Variance) Close() error {
	return nil
}

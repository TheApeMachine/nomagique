package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
ZScore tracks adaptive scale for a normalized surprise score.
*/
type ZScore struct {
	artifact *datura.Artifact
}

/*
NewZScore returns a z-score stage ready to bootstrap from its first observation.
*/
func NewZScore() *ZScore {
	return &ZScore{
		artifact: datura.Acquire("zscore", datura.APPJSON).RetainStageAttributes(),
	}
}

func (surprise *ZScore) Read(p []byte) (int, error) {
	sample := datura.Peek[float64](surprise.artifact, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return surprise.artifact.Read(p)
	}

	anchor := datura.Peek[float64](surprise.artifact, "anchor")
	hasAnchor := anchor != 0 && !math.IsNaN(anchor) && !math.IsInf(anchor, 0)

	output := datura.Peek[datura.Map[float64]](surprise.artifact, "output")

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

		surprise.artifact.Poke(output, "output")

		return surprise.artifact.Read(p)
	}

	output["min"] = math.Min(output["min"], sample)
	output["max"] = math.Max(output["max"], sample)

	span := output["max"] - output["min"]

	if span == 0 {
		output["prev"] = sample
		surprise.artifact.Poke(output, "output")

		return surprise.artifact.Read(p)
	}

	delta := math.Abs(sample - output["prev"])
	output["rate"] = delta / span
	level := output["mean"]

	if hasAnchor {
		level = anchor
	}

	deviation := sample - level

	if !hasAnchor {
		output["mean"] += output["rate"] * (sample - output["mean"])
	}

	output["var"] += output["rate"] * (deviation*deviation - output["var"])
	output["prev"] = sample

	if output["var"] <= 0 {
		output["value"] = 0
		surprise.artifact.Poke(output, "output")

		return surprise.artifact.Read(p)
	}

	output["value"] = deviation / math.Sqrt(output["var"])

	surprise.artifact.Poke(output, "output")

	return surprise.artifact.Read(p)
}

func (surprise *ZScore) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](surprise.artifact, "output") == nil

	surprise.artifact.Clear("sample")

	n, err := surprise.artifact.Write(p)

	if bootstrap {
		surprise.artifact.Clear("output")
	}

	return n, err
}

func (surprise *ZScore) Close() error {
	return nil
}

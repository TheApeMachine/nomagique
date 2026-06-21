package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
ZScore tracks adaptive scale for a normalized surprise score.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type ZScore struct {
	artifact *datura.Artifact
}

/*
NewZScore returns a z-score stage wired from config attributes on the artifact.
*/
func NewZScore(artifact *datura.Artifact) *ZScore {
	artifact.Inspect("adaptive", "zscore", "NewZScore()")

	return &ZScore{
		artifact: artifact,
	}
}

func (surprise *ZScore) Write(payload []byte) (int, error) {
	surprise.artifact.WithPayload(payload)
	return len(payload), nil
}

func (surprise *ZScore) Read(payload []byte) (int, error) {
	state := datura.Acquire("zscore-state", datura.APPJSON)
	state.Inspect("adaptive", "zscore", "Read()", "p")

	if _, err := state.Write(surprise.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	sample := datura.Peek[float64](state, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return state.Read(payload)
	}

	anchor := datura.Peek[float64](state, "anchor")

	if anchor == 0 {
		anchor = datura.Peek[float64](surprise.artifact, "anchor")
	}

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
		surprise.artifact.Poke(output, "output")
		state.MergeOutput("value", output["value"])
		state.Merge("root", "output")
		state.Merge("inputs", []string{"value"})
		return state.Read(payload)
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
		state.MergeOutput("value", output["value"])
		state.Merge("root", "output")
		state.Merge("inputs", []string{"value"})
		return state.Read(payload)
	}

	output["value"] = deviation / math.Sqrt(output["var"])

	surprise.artifact.Poke(output, "output")
	state.MergeOutput("value", output["value"])
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(payload)
}

func (surprise *ZScore) Close() error {
	return nil
}

package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/statistic"
)

/*
ZScore tracks adaptive scale for a normalized surprise score.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type ZScore struct {
	artifact     *datura.Artifact
	bootstrapped bool
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

	features := statistic.SnapshotFeatures(state)
	sampleKey := statistic.WireInputKey(surprise.artifact, state, "sample")
	sample, err := statistic.WireScalar(surprise.artifact, state, sampleKey)

	if err != nil {
		return 0, err
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"zscore: sample is non-finite",
			nil,
		))
	}

	anchor, hasAnchor := surprise.anchor(state)

	output := datura.Map[float64]{
		"mean": datura.Peek[float64](surprise.artifact, "output", "mean"),
		"var":  datura.Peek[float64](surprise.artifact, "output", "var"),
		"prev": datura.Peek[float64](surprise.artifact, "output", "prev"),
		"min":  datura.Peek[float64](surprise.artifact, "output", "min"),
		"max":  datura.Peek[float64](surprise.artifact, "output", "max"),
		"rate": datura.Peek[float64](surprise.artifact, "output", "rate"),
	}

	if !surprise.bootstrapped {
		output = datura.Map[float64]{
			"mean": sample,
			"var":  0,
			"prev": sample,
			"min":  sample,
			"max":  sample,
			"rate": 0,
		}

		surprise.bootstrapped = true
		surprise.artifact.Poke(output, "output")

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"zscore: insufficient samples",
			nil,
		))
	}

	output["min"] = math.Min(output["min"], sample)
	output["max"] = math.Max(output["max"], sample)

	span := output["max"] - output["min"]

	if span == 0 {
		output["prev"] = sample
		surprise.artifact.Poke(output, "output")

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"zscore: sample span is zero",
			nil,
		))
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
		surprise.artifact.Poke(output, "output")

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"zscore: variance is not positive",
			nil,
		))
	}

	output["value"] = deviation / math.Sqrt(output["var"])

	if math.IsNaN(output["value"]) || math.IsInf(output["value"], 0) {
		surprise.artifact.Poke(output, "output")

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"zscore: output value is non-finite",
			nil,
		))
	}

	surprise.artifact.Poke(output, "output")
	state.MergeOutput("value", output["value"])
	features.Restore(state)
	state.Merge("root", "output")

	if len(datura.Peek[[]string](state, "inputs")) == 0 {
		state.Merge("inputs", []string{"value"})
	}

	return state.Read(payload)
}

func (surprise *ZScore) anchor(state *datura.Artifact) (float64, bool) {
	anchorMode := datura.Peek[string](surprise.artifact, "anchorMode")

	if anchorMode == "explicit" || anchorMode == "fixed" {
		anchor := datura.Peek[float64](state, "anchor")

		if anchor == 0 {
			anchor = datura.Peek[float64](surprise.artifact, "anchor")
		}

		if math.IsNaN(anchor) || math.IsInf(anchor, 0) {
			return 0, false
		}

		return anchor, true
	}

	body := datura.As[datura.Map[any]](state)

	if body != nil {
		if _, present := body["anchor"]; present {
			anchor := datura.Peek[float64](state, "anchor")

			if math.IsNaN(anchor) || math.IsInf(anchor, 0) {
				return 0, false
			}

			return anchor, true
		}
	}

	anchor := datura.Peek[float64](surprise.artifact, "anchor")

	if anchor != 0 && !math.IsNaN(anchor) && !math.IsInf(anchor, 0) {
		return anchor, true
	}

	return 0, false
}

func (surprise *ZScore) Close() error {
	return nil
}

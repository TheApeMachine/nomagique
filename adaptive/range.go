package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/statistic"
)

/*
Range tracks the running span of observed samples.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Range struct {
	artifact     *datura.Artifact
	bootstrapped bool
}

/*
NewRange returns a range stage wired from config attributes on the artifact.
*/
func NewRange(artifact *datura.Artifact) *Range {
	artifact.Inspect("adaptive", "range", "NewRange()")

	return &Range{
		artifact: artifact,
	}
}

func (extent *Range) Write(payload []byte) (int, error) {
	extent.artifact.WithPayload(payload)
	return len(payload), nil
}

func (extent *Range) Read(payload []byte) (int, error) {
	state := datura.Acquire("range-state", datura.APPJSON)
	state.Inspect("adaptive", "range", "Read()", "p")

	if _, err := state.Write(extent.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	features := statistic.SnapshotFeatures(state)
	sampleKey, err := statistic.WireInputKey(extent.artifact, state)

	if err != nil {
		return 0, err
	}

	sample, err := statistic.WireScalar(extent.artifact, state, sampleKey)

	if err != nil {
		return 0, err
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"range: sample is non-finite",
			nil,
		))
	}

	output := datura.Map[float64]{
		"min":   datura.Peek[float64](extent.artifact, "output", "min"),
		"max":   datura.Peek[float64](extent.artifact, "output", "max"),
		"value": datura.Peek[float64](extent.artifact, "output", "value"),
	}

	if !extent.bootstrapped {
		output = datura.Map[float64]{
			"min":   sample,
			"max":   sample,
			"value": 0,
		}

		extent.bootstrapped = true
		extent.artifact.Poke(output, "output")

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"range: insufficient samples",
			nil,
		))
	}

	output["min"] = math.Min(output["min"], sample)
	output["max"] = math.Max(output["max"], sample)

	span := output["max"] - output["min"]

	if span == 0 {
		extent.artifact.Poke(output, "output")

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"range: sample span is zero",
			nil,
		))
	}

	output["value"] = span

	extent.artifact.Poke(output, "output")
	state.MergeOutput("value", output["value"])
	features.Restore(state)
	state.Merge("root", "output")

	if len(datura.Peek[[]string](state, "inputs")) == 0 {
		state.Merge("inputs", []string{"value"})
	}

	return state.Read(payload)
}

func (extent *Range) Close() error {
	return nil
}

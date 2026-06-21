package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
Range tracks the running span of observed samples.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Range struct {
	artifact *datura.Artifact
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
	output := datura.Peek[datura.Map[float64]](extent.artifact, "output")

	extent.artifact.WithPayload(payload)

	if output != nil {
		extent.artifact.Merge("output", output)
	}

	return len(payload), nil
}

func (extent *Range) Read(payload []byte) (int, error) {
	state := datura.Acquire("range-state", datura.APPJSON)
	state.Inspect("adaptive", "range", "Read()", "p")

	if _, err := state.Write(extent.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	sample := datura.Peek[float64](state, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return state.Read(payload)
	}

	output := datura.Peek[datura.Map[float64]](extent.artifact, "output")

	if output == nil {
		output = datura.Map[float64]{
			"min":   sample,
			"max":   sample,
			"value": 0,
		}

		extent.artifact.Merge("output", output)
		state.MergeOutput("value", output["value"])
		state.Merge("root", "output")
		state.Merge("inputs", []string{"value"})
		return state.Read(payload)
	}

	output["min"] = math.Min(output["min"], sample)
	output["max"] = math.Max(output["max"], sample)
	output["value"] = output["max"] - output["min"]

	extent.artifact.Merge("output", output)
	state.MergeOutput("value", output["value"])
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(payload)
}

func (extent *Range) Close() error {
	return nil
}

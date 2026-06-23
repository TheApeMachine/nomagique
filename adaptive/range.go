package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Range tracks the running span of observed samples.
The constructor artifact holds config; Write buffers inbound payload.
*/
type Range struct {
	artifact     *datura.Artifact
	bootstrapped bool
	min          float64
	max          float64
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

func (extent *Range) Write(p []byte) (int, error) {
	extent.artifact.WithPayload(p)
	return len(p), nil
}

func (extent *Range) Read(payload []byte) (int, error) {
	state := datura.Acquire("range-state", datura.APPJSON)
	state.Inspect("adaptive", "range", "Read()", "p")

	if _, err := state.Write(extent.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"range: state write failed",
			err,
		))
	}

	root := datura.Peek[string](state, "root")
	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		inputs = []string{"sample"}
	}

	for _, input := range inputs {
		sample := datura.Peek[float64](state, root, input)

		if root == "" {
			sample = datura.Peek[float64](state, input)
		}

		if math.IsNaN(sample) || math.IsInf(sample, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"range: sample is non-finite",
				nil,
			))
		}

		if !extent.bootstrapped {
			extent.min = sample
			extent.max = sample
			extent.bootstrapped = true

			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"range: insufficient samples",
				nil,
			))
		}

		extent.min = math.Min(extent.min, sample)
		extent.max = math.Max(extent.max, sample)

		span := extent.max - extent.min

		if span == 0 {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"range: sample span is zero",
				nil,
			))
		}

		state.Poke("output", "root")
		state.Poke([]string{"value"}, "inputs")
		state.MergeOutput("value", span)
	}

	return state.Read(payload)
}

func (extent *Range) Close() error {
	return nil
}

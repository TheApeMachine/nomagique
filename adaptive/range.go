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
	artifact *datura.Artifact
	min      float64
	max      float64
	count    int
}

/*
NewRange returns a range stage wired from config attributes on the artifact.
*/
func NewRange(artifact *datura.Artifact) *Range {
	return &Range{
		artifact: artifact,
	}
}

func (extent *Range) Read(payload []byte) (int, error) {
	state := datura.Acquire("range-state", datura.APPJSON)

	if _, err := state.Write(extent.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"range: state write failed",
			err,
		))
	}

	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"range: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"range: inputs required",
			nil,
		))
	}

	for index, input := range inputs {
		var sample float64

		if rootKey == "features" {
			features := datura.Peek[[]float64](state, rootKey)

			if index >= len(features) {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"range: feature index out of range",
					nil,
				))
			}

			sample = features[index]
		}

		if rootKey != "features" {
			sample = datura.Peek[float64](state, rootKey, input)
		}

		if math.IsNaN(sample) || math.IsInf(sample, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"range: sample is non-finite",
				nil,
			))
		}

		if extent.count == 0 {
			extent.min = sample
			extent.max = sample
			extent.count = 1
		} else {
			extent.min = math.Min(extent.min, sample)
			extent.max = math.Max(extent.max, sample)
			extent.count++
		}

		span := extent.max - extent.min

		if span == 0 {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"range: sample span is zero",
				nil,
			))
		}

		state.MergeOutput("value", span)
	}

	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")

	return state.Read(payload)
}

func (extent *Range) Write(p []byte) (int, error) {
	extent.artifact.WithPayload(p)
	return len(p), nil
}

func (extent *Range) Close() error {
	return nil
}

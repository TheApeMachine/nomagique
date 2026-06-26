package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
ZScore tracks adaptive scale for a normalized surprise score.
The constructor artifact holds config; Write buffers inbound payload.
*/
type ZScore struct {
	artifact *datura.Artifact
	mean     float64
	variance float64
	prev     float64
	min      float64
	max      float64
	count    int
}

/*
NewZScore returns a z-score stage wired from config attributes on the artifact.
*/
func NewZScore(artifact *datura.Artifact) *ZScore {
	return &ZScore{
		artifact: artifact,
	}
}

func (surprise *ZScore) Read(payload []byte) (int, error) {
	state := datura.Acquire("zscore-state", datura.APPJSON)

	if _, err := state.Unpack(surprise.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"zscore: state write failed",
			err,
		))
	}

	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"zscore: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"zscore: inputs required",
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
					"zscore: feature index out of range",
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
				"zscore: sample is non-finite",
				nil,
			))
		}

		anchorMode := datura.Peek[string](surprise.artifact, "anchorMode")
		hasAnchor := false
		anchor := 0.0
		rawWireAnchor := datura.Peek[any](state, "anchor")

		if anchorMode == "explicit" || anchorMode == "fixed" {
			anchor = datura.Peek[float64](surprise.artifact, "anchor")

			if math.IsNaN(anchor) || math.IsInf(anchor, 0) {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"zscore: anchor is non-finite",
					nil,
				))
			}

			hasAnchor = true

			if rawWireAnchor != nil {
				anchor = datura.Peek[float64](state, "anchor")

				if math.IsNaN(anchor) || math.IsInf(anchor, 0) {
					return 0, errnie.Error(errnie.Err(
						errnie.Validation,
						"zscore: anchor is non-finite",
						nil,
					))
				}
			}
		}

		if !hasAnchor && rawWireAnchor != nil {
			anchor = datura.Peek[float64](state, "anchor")

			if math.IsNaN(anchor) || math.IsInf(anchor, 0) {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"zscore: anchor is non-finite",
					nil,
				))
			}

			hasAnchor = true
		}

		if surprise.count == 0 {
			surprise.mean = sample
			surprise.variance = 0
			surprise.prev = sample
			surprise.min = sample
			surprise.max = sample
			surprise.count = 1
		} else {
			surprise.min = math.Min(surprise.min, sample)
			surprise.max = math.Max(surprise.max, sample)
			surprise.count++
		}

		span := surprise.max - surprise.min

		if span == 0 {
			surprise.prev = sample

			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"zscore: sample span is zero",
				nil,
			))
		}

		rate := math.Abs(sample-surprise.prev) / span
		level := surprise.mean

		if hasAnchor {
			level = anchor
		}

		deviation := sample - level

		if !hasAnchor {
			surprise.mean += rate * (sample - surprise.mean)
		}

		surprise.variance += rate * (deviation*deviation - surprise.variance)
		surprise.prev = sample

		if surprise.variance <= 0 {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"zscore: variance is not positive",
				nil,
			))
		}

		value := deviation / math.Sqrt(surprise.variance)

		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"zscore: output value is non-finite",
				nil,
			))
		}

		state.MergeOutput("value", value)
	}

	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")

	return state.PackInto(payload)
}

func (surprise *ZScore) Write(p []byte) (int, error) {
	surprise.artifact.WithPayload(p)
	return len(p), nil
}

func (surprise *ZScore) Close() error {
	return nil
}

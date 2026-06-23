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
	artifact     *datura.Artifact
	bootstrapped bool
	mean         float64
	variance     float64
	prev         float64
	min          float64
	max          float64
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

func (surprise *ZScore) Write(p []byte) (int, error) {
	surprise.artifact.WithPayload(p)
	return len(p), nil
}

func (surprise *ZScore) Read(payload []byte) (int, error) {
	state := datura.Acquire("zscore-state", datura.APPJSON)
	state.Inspect("adaptive", "zscore", "Read()", "p")

	if _, err := state.Write(surprise.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"zscore: state write failed",
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
				"zscore: sample is non-finite",
				nil,
			))
		}

		anchorMode := datura.Peek[string](surprise.artifact, "anchorMode")
		hasAnchor := false
		anchor := 0.0

		if anchorMode == "explicit" || anchorMode == "fixed" {
			anchor = datura.Peek[float64](state, "anchor")

			if anchor == 0 {
				anchor = datura.Peek[float64](surprise.artifact, "anchor")
			}

			if math.IsNaN(anchor) || math.IsInf(anchor, 0) {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"zscore: anchor is non-finite",
					nil,
				))
			}

			hasAnchor = true
		}

		if !hasAnchor {
			body := datura.As[datura.Map[any]](state)

			if body != nil {
				if _, present := body["anchor"]; present {
					anchor = datura.Peek[float64](state, "anchor")

					if !math.IsNaN(anchor) && !math.IsInf(anchor, 0) {
						hasAnchor = true
					}
				}
			}
		}

		if !hasAnchor {
			anchor = datura.Peek[float64](surprise.artifact, "anchor")

			if anchor != 0 && !math.IsNaN(anchor) && !math.IsInf(anchor, 0) {
				hasAnchor = true
			}
		}

		if !surprise.bootstrapped {
			surprise.mean = sample
			surprise.variance = 0
			surprise.prev = sample
			surprise.min = sample
			surprise.max = sample
			surprise.bootstrapped = true

			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"zscore: insufficient samples",
				nil,
			))
		}

		surprise.min = math.Min(surprise.min, sample)
		surprise.max = math.Max(surprise.max, sample)

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

		state.Poke("output", "root")
		state.Poke([]string{"value"}, "inputs")
		state.MergeOutput("value", value)
	}

	return state.Read(payload)
}

func (surprise *ZScore) Close() error {
	return nil
}

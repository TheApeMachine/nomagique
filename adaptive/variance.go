package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Variance tracks an adaptive mean and variance from the observed sample stream.
The constructor artifact holds config; Write buffers inbound payload.
*/
type Variance struct {
	artifact *datura.Artifact
	mean     float64
	variance float64
	prev     float64
	min      float64
	max      float64
	count    int
}

/*
NewVariance returns a variance stage wired from config attributes on the artifact.
*/
func NewVariance(artifact *datura.Artifact) *Variance {
	return &Variance{
		artifact: artifact,
	}
}

func (variance *Variance) Read(payload []byte) (int, error) {
	state := datura.Acquire("variance-state", datura.APPJSON)

	if _, err := state.Unpack(variance.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"variance: state write failed",
			err,
		))
	}

	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"variance: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"variance: inputs required",
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
					"variance: feature index out of range",
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
				"variance: sample is non-finite",
				nil,
			))
		}

		if variance.count == 0 {
			variance.mean = sample
			variance.variance = 0
			variance.prev = sample
			variance.min = sample
			variance.max = sample
			variance.count = 1
		} else {
			variance.min = math.Min(variance.min, sample)
			variance.max = math.Max(variance.max, sample)
			variance.count++
		}

		span := variance.max - variance.min

		if span == 0 {
			variance.prev = sample
			mergeStageOutput(state, 0, false)
			continue
		}

		rate := math.Abs(sample-variance.prev) / span
		deviation := sample - variance.mean
		variance.mean += rate * (sample - variance.mean)
		variance.variance += rate * (deviation*deviation - variance.variance)
		variance.prev = sample

		if variance.variance <= 0 {
			mergeStageOutput(state, 0, false)
			continue
		}

		mergeStageOutput(state, variance.variance, true)
	}

	return state.PackInto(payload)
}

func (variance *Variance) Write(p []byte) (int, error) {
	variance.artifact.WithPlaintextPayload(p)
	return len(p), nil
}

func (variance *Variance) Close() error {
	return nil
}

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
	artifact     *datura.Artifact
	bootstrapped bool
	mean         float64
	variance     float64
	prev         float64
	min          float64
	max          float64
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

	if _, err := state.Write(variance.artifact.DecryptPayload()); err != nil {
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

		if !variance.bootstrapped {
			variance.mean = sample
			variance.variance = 0
			variance.prev = sample
			variance.min = sample
			variance.max = sample
			variance.bootstrapped = true
			state.MergeOutput("value", 0)

			break
		}

		variance.min = math.Min(variance.min, sample)
		variance.max = math.Max(variance.max, sample)

		span := variance.max - variance.min

		if span == 0 {
			variance.prev = sample

			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"variance: sample span is zero",
				nil,
			))
		}

		rate := math.Abs(sample-variance.prev) / span
		deviation := sample - variance.mean
		variance.mean += rate * (sample - variance.mean)
		variance.variance += rate * (deviation*deviation - variance.variance)
		variance.prev = sample

		if variance.variance <= 0 {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"variance: variance is not positive",
				nil,
			))
		}

		state.MergeOutput("value", variance.variance)
	}

	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")

	return state.Read(payload)
}

func (variance *Variance) Write(p []byte) (int, error) {
	variance.artifact.WithPayload(p)
	return len(p), nil
}

func (variance *Variance) Close() error {
	return nil
}

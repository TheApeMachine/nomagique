package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Entropy computes normalized Shannon entropy over retained history.
*/
type Entropy struct {
	artifact *datura.Artifact
}

/*
NewEntropy returns an entropy stage wired from config attributes on the artifact.
*/
func NewEntropy(artifact *datura.Artifact) *Entropy {
	return &Entropy{
		artifact: artifact,
	}
}

func (entropy *Entropy) Read(payload []byte) (int, error) {
	state := datura.Acquire("entropy-state", datura.APPJSON)

	if _, err := state.Unpack(entropy.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"entropy: state write failed",
			err,
		))
	}

	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"entropy: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"entropy: inputs required",
			nil,
		))
	}

	configInput := datura.Peek[string](entropy.artifact, "input")

	if configInput == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"entropy: input required",
			nil,
		))
	}

	outputKey := datura.Peek[string](entropy.artifact, "outputKey")

	if outputKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"entropy: outputKey required",
			nil,
		))
	}

	var sample float64
	found := false

	for index, input := range inputs {
		if input != configInput {
			continue
		}

		if rootKey == "features" {
			features := datura.Peek[[]float64](state, rootKey)

			if index >= len(features) {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"entropy: feature index out of range",
					nil,
				))
			}

			sample = features[index]
		}

		if rootKey != "features" {
			sample = datura.Peek[float64](state, rootKey, input)
		}

		found = true
	}

	if !found {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"entropy: input not in inputs",
			nil,
		))
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) || sample < 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"entropy: sample must be finite and non-negative",
			nil,
		))
	}

	history := datura.Peek[[]float64](entropy.artifact, "history")
	history = append(history, sample)
	entropy.artifact.Poke(history, "history")

	total := 0.0

	for _, value := range history {
		total += value
	}

	value := 0.0

	if total > 0 {
		probabilityFloor := total / float64(len(history))
		entropySum := 0.0

		for _, entry := range history {
			probability := entry / total

			if probability <= 0 {
				continue
			}

			entropySum -= probability * math.Log(probability)
		}

		maxEntropy := math.Log(float64(len(history)))

		if maxEntropy > 0 {
			value = entropySum / maxEntropy
		}

		if probabilityFloor <= 0 {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"entropy: probability floor is non-positive",
				nil,
			))
		}
	}

	state.MergeOutput(outputKey, value)
	state.Poke("output", "root")
	state.Poke([]string{outputKey}, "inputs")

	return state.PackInto(payload)
}

func (entropy *Entropy) Write(payload []byte) (int, error) {
	entropy.artifact.WithPayload(payload)
	return len(payload), nil
}

func (entropy *Entropy) Close() error {
	return nil
}

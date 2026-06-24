package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
PriceRing publishes the configured input sample on the outbound wire.
*/
type PriceRing struct {
	artifact *datura.Artifact
}

/*
NewPriceRing returns a sample ring stage wired from config attributes on the artifact.
*/
func NewPriceRing(artifact *datura.Artifact) *PriceRing {
	return &PriceRing{
		artifact: artifact,
	}
}

func (priceRing *PriceRing) Read(payload []byte) (int, error) {
	state := datura.Acquire("price-ring-state", datura.APPJSON)

	if _, err := state.Write(priceRing.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"price-ring: state write failed",
			err,
		))
	}

	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"price-ring: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"price-ring: inputs required",
			nil,
		))
	}

	stageKey := datura.Peek[string](priceRing.artifact, "block")

	if stageKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"price-ring: block required",
			nil,
		))
	}

	configInput := datura.Peek[string](priceRing.artifact, stageKey, "input")

	if configInput == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"price-ring: input required",
			nil,
		))
	}

	var sample float64
	sampleFound := false

	for index, input := range inputs {
		if input != configInput {
			continue
		}

		if rootKey == "features" {
			featureSlice := datura.Peek[[]float64](state, rootKey)

			if index >= len(featureSlice) {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"price-ring: feature index out of range",
					nil,
				))
			}

			sample = featureSlice[index]
		}

		if rootKey != "features" {
			sample = datura.Peek[float64](state, rootKey, input)
		}

		sampleFound = true
	}

	if !sampleFound {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"price-ring: input not in inputs",
			nil,
		))
	}

	if sample <= 0 || math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"price-ring: sample is non-positive or non-finite",
			nil,
		))
	}

	state.MergeOutput(configInput, sample)
	state.Poke("output", "root")
	state.Poke([]string{configInput}, "inputs")

	return state.Read(payload)
}

func (priceRing *PriceRing) Write(payload []byte) (int, error) {
	priceRing.artifact.WithPayload(payload)
	return len(payload), nil
}

func (priceRing *PriceRing) Close() error {
	return nil
}

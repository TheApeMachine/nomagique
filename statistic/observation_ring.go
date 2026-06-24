package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
ObservationRing retains a bounded history of positive observations.
*/
type ObservationRing struct {
	artifact *datura.Artifact
}

/*
NewObservationRing returns an observation-ring stage wired from config attributes on the artifact.
*/
func NewObservationRing(artifact *datura.Artifact) *ObservationRing {
	return &ObservationRing{
		artifact: artifact,
	}
}

func (observationRing *ObservationRing) Read(payload []byte) (int, error) {
	state := datura.Acquire("observation-ring-state", datura.APPJSON)

	if _, err := state.Write(observationRing.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"observation-ring: state write failed",
			err,
		))
	}

	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"observation-ring: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"observation-ring: inputs required",
			nil,
		))
	}

	configInput := datura.Peek[string](observationRing.artifact, "input")

	if configInput == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"observation-ring: input required",
			nil,
		))
	}

	outputKey := datura.Peek[string](observationRing.artifact, "outputKey")

	if outputKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"observation-ring: outputKey required",
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
					"observation-ring: feature index out of range",
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
			"observation-ring: input not in inputs",
			nil,
		))
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) || sample <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"observation-ring: sample must be finite and positive",
			nil,
		))
	}

	history := datura.Peek[[]float64](observationRing.artifact, "history")
	history = append(history, sample)

	capacity := int(datura.Peek[float64](observationRing.artifact, "config", "capacity"))

	if capacity <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"observation-ring: capacity required",
			nil,
		))
	}

	if len(history) > capacity {
		history = history[len(history)-capacity:]
	}

	observationRing.artifact.Poke(history, "history")

	state.MergeOutput(outputKey, sample)
	state.Poke("output", "root")
	state.Poke([]string{outputKey}, "inputs")

	return state.Read(payload)
}

func (observationRing *ObservationRing) Write(payload []byte) (int, error) {
	observationRing.artifact.WithPayload(payload)
	return len(payload), nil
}

func (observationRing *ObservationRing) Close() error {
	return nil
}

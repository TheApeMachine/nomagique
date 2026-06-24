package learning

import (
	"github.com/bytedance/sonic"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

func weightStateFromArtifact(artifact *datura.Artifact) weightResetFields {
	count := datura.Peek[float64](artifact, "output", "count")

	return weightResetFields{
		Trust: datura.Peek[float64](artifact, "output", "trust"),
		Prev:  datura.Peek[float64](artifact, "output", "prev"),
		Min:   datura.Peek[float64](artifact, "output", "min"),
		Max:   datura.Peek[float64](artifact, "output", "max"),
		Rate:  datura.Peek[float64](artifact, "output", "rate"),
		Count: count,
	}
}

type weightResetFields struct {
	Trust float64
	Prev  float64
	Min   float64
	Max   float64
	Rate  float64
	Count float64
}

func pokeWeightState(artifact *datura.Artifact, fields *weightResetFields, value float64) {
	artifact.Poke(fields.Trust, "output", "trust")
	artifact.Poke(fields.Prev, "output", "prev")
	artifact.Poke(fields.Min, "output", "min")
	artifact.Poke(fields.Max, "output", "max")
	artifact.Poke(fields.Rate, "output", "rate")
	artifact.Poke(fields.Count, "output", "count")
	artifact.Poke(value, "output", "value")
}

func wirePair(
	artifact *datura.Artifact,
	state *datura.Artifact,
	stage string,
) (float64, float64, error) {
	sampleKey := datura.Peek[string](artifact, "sampleKey")

	if sampleKey == "" {
		return 0, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			stage+": sampleKey required",
			nil,
		))
	}

	pairedKey := datura.Peek[string](artifact, "pairedKey")

	if pairedKey == "" {
		return 0, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			stage+": pairedKey required",
			nil,
		))
	}

	wireRoot := datura.Peek[string](state, "root")

	if wireRoot == "" {
		return 0, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			stage+": root required",
			nil,
		))
	}

	wireInputs := datura.Peek[[]string](state, "inputs")

	if len(wireInputs) == 0 {
		return 0, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			stage+": inputs required",
			nil,
		))
	}

	var predicted float64
	predictedFound := false

	for wireIndex, wireInput := range wireInputs {
		if wireInput != sampleKey {
			continue
		}

		if wireRoot == "features" {
			features := datura.Peek[[]float64](state, wireRoot)

			if wireIndex >= len(features) {
				return 0, 0, errnie.Error(errnie.Err(
					errnie.Validation,
					stage+": feature index out of range",
					nil,
				))
			}

			predicted = features[wireIndex]
		}

		if wireRoot != "features" {
			predicted = datura.Peek[float64](state, wireRoot, wireInput)
		}

		predictedFound = true
	}

	if !predictedFound {
		return 0, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			stage+": sampleKey not in inputs",
			nil,
		))
	}

	var actual float64
	actualFound := false

	for wireIndex, wireInput := range wireInputs {
		if wireInput != pairedKey {
			continue
		}

		if wireRoot == "features" {
			features := datura.Peek[[]float64](state, wireRoot)

			if wireIndex >= len(features) {
				return 0, 0, errnie.Error(errnie.Err(
					errnie.Validation,
					stage+": feature index out of range",
					nil,
				))
			}

			actual = features[wireIndex]
		}

		if wireRoot != "features" {
			actual = datura.Peek[float64](state, wireRoot, wireInput)
		}

		actualFound = true
	}

	if !actualFound {
		return 0, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			stage+": pairedKey not in inputs",
			nil,
		))
	}

	parsedPredicted, parsedActual, err := parsePredictedActual(predicted, []float64{actual})

	if err != nil {
		return 0, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			stage+": unable to parse predicted and actual pair",
			err,
		))
	}

	return parsedPredicted, parsedActual, nil
}

func attributeKeyPresent(artifact *datura.Artifact, key string) bool {
	rawAttributes, err := artifact.Attributes()

	if err == nil && len(rawAttributes) > 0 {
		node, err := sonic.Get(rawAttributes, key)

		if err == nil && node.Exists() {
			return true
		}
	}

	payload := artifact.DecryptPayload()

	if err != nil || len(payload) == 0 {
		return false
	}

	node, err := sonic.Get(payload, key)

	return err == nil && node.Exists()
}

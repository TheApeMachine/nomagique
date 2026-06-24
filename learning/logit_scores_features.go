package learning

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

func (logitScores *LogitScores) featureValues(
	state *datura.Artifact,
	order []string,
) (map[string]float64, error) {
	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"logit-scores: root required",
			nil,
		))
	}

	features := make(map[string]float64, len(order))

	for _, key := range order {
		wireKey := datura.Peek[string](logitScores.config, key, "source")

		if wireKey == "" {
			wireKey = key
		}

		outputMap := datura.Peek[map[string]any](state, rootKey)

		if _, ok := outputMap[wireKey]; !ok {
			return nil, errnie.Error(errnie.Err(
				errnie.Validation,
				"logit-scores: score key missing from "+rootKey,
				nil,
			))
		}

		value := datura.Peek[float64](state, rootKey, wireKey)

		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, errnie.Error(errnie.Err(
				errnie.Validation,
				"logit-scores: score is non-finite",
				nil,
			))
		}

		features[key] = value
	}

	return features, nil
}

func (logitScores *LogitScores) readWireValue(
	state *datura.Artifact,
	wireKey string,
) (float64, error) {
	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"logit-scores: root required",
			nil,
		))
	}

	outputMap := datura.Peek[map[string]any](state, rootKey)

	if _, ok := outputMap[wireKey]; !ok {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"logit-scores: score key missing from "+rootKey,
			nil,
		))
	}

	value := datura.Peek[float64](state, rootKey, wireKey)

	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"logit-scores: score is non-finite",
			nil,
		))
	}

	return value, nil
}

func (logitScores *LogitScores) centeredFeatures(
	features map[string]float64,
	scales map[string]float64,
) map[string]float64 {
	centered := make(map[string]float64, len(features))

	for key, feature := range features {
		centered[key] = logitScores.centeredFeature(key, feature, scales[key])
	}

	return centered
}

func (logitScores *LogitScores) centeredFeature(
	stageKey string,
	feature float64,
	scale float64,
) float64 {
	if datura.Peek[string](logitScores.config, stageKey, "centerMode") != "median" {
		return feature
	}

	centered := feature - scale

	if centered < 0 {
		return 0
	}

	return centered
}

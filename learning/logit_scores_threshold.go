package learning

import (
	"fmt"
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/statistic"
)

func (logitScores *LogitScores) resolveThreshold(
	features map[string]float64,
	state *datura.Artifact,
) (float64, error) {
	thresholdSource := datura.Peek[string](logitScores.config, "threshold", "source")

	if thresholdSource != "" {
		rootKey := datura.Peek[string](logitScores.config, "root")

		if rootKey == "" {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"logit-scores: config root required for threshold source",
				nil,
			))
		}

		var value float64
		valueFound := false
		wireInputs := datura.Peek[[]string](state, "inputs")

		for wireIndex, wireInput := range wireInputs {
			if wireInput != thresholdSource {
				continue
			}

			if rootKey == "features" {
				features := datura.Peek[[]float64](state, rootKey)

				if wireIndex >= len(features) {
					return 0, errnie.Error(errnie.Err(
						errnie.Validation,
						"logit-scores: feature index out of range",
						nil,
					))
				}

				value = features[wireIndex]
			}

			if rootKey != "features" {
				value = datura.Peek[float64](state, rootKey, wireInput)
			}

			valueFound = true
		}

		if !valueFound {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"logit-scores: threshold source not in inputs",
				nil,
			))
		}

		if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				fmt.Sprintf(
					"logit-scores: threshold source %q is non-positive",
					thresholdSource,
				),
				nil,
			))
		}

		return value, nil
	}

	configured := datura.Peek[float64](logitScores.config, "threshold")

	if configured > 0 {
		return configured, nil
	}

	return logitScores.resolveThresholdFromHistory(features)
}

func (logitScores *LogitScores) resolveThresholdFromHistory(
	features map[string]float64,
) (float64, error) {
	magnitude := 0.0

	for _, feature := range features {
		magnitude += math.Abs(feature)
	}

	prior := datura.Peek[[]float64](logitScores.config, "output", "thresholdSamples")
	samples := append(prior, magnitude)
	logitScores.config.Poke(samples, "output", "thresholdSamples")

	threshold, ok := statistic.MedianOf(samples)

	if !ok || threshold <= 0 || math.IsNaN(threshold) || math.IsInf(threshold, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"logit-scores: threshold could not be derived from feature history",
			nil,
		))
	}

	return threshold, nil
}

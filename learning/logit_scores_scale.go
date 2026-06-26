package learning

import (
	"fmt"
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/statistic"
)

func (logitScores *LogitScores) featureScales(
	order []string,
	features map[string]float64,
	state *datura.Artifact,
) (map[string]float64, error) {
	scales := make(map[string]float64, len(order))

	for _, key := range order {
		scale, err := logitScores.resolveFeatureScale(key, features[key], state)

		if err != nil {
			return nil, err
		}

		scales[key] = scale
	}

	return scales, nil
}

func (logitScores *LogitScores) resolveFeatureScale(
	stageKey string,
	feature float64,
	state *datura.Artifact,
) (float64, error) {
	_ = state

	configured := datura.Peek[float64](logitScores.config, stageKey, "scale")

	if configured > 0 {
		return configured, nil
	}

	samples := datura.Peek[[]float64](logitScores.config, "output", "scaleSamples", stageKey)
	samples = append(samples, math.Abs(feature))
	logitScores.config.Poke(samples, "output", "scaleSamples", stageKey)

	derived, ok := statistic.MedianOf(samples)

	if !ok {
		derived = 0
	}

	if derived <= 0 {
		derived, ok = positiveMedian(samples)

		if !ok {
			derived = 0
		}
	}

	scaleMode := datura.Peek[string](logitScores.config, stageKey, "scaleMode")

	if scaleMode == "median" {
		if derived <= 0 || math.IsNaN(derived) || math.IsInf(derived, 0) {
			if feature == 0 {
				return 0, nil
			}

			return 0, errnie.Err(
				errnie.Validation,
				fmt.Sprintf("logit-scores: median scale for %q requires positive history", stageKey),
				nil,
			)
		}

		return derived, nil
	}

	scale := math.Max(derived, math.Abs(feature))

	if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			fmt.Sprintf("logit-scores: scale for %q could not be derived", stageKey),
			nil,
		))
	}

	return scale, nil
}

func (logitScores *LogitScores) resolveCompositeScale(
	stageKey string,
) (float64, error) {
	leftKey := datura.Peek[string](logitScores.config, stageKey, "leftKey")
	rightKey := datura.Peek[string](logitScores.config, stageKey, "rightKey")

	if leftKey == "" || rightKey == "" {
		leftKey = datura.Peek[string](logitScores.config, "joint", "leftKey")
		rightKey = datura.Peek[string](logitScores.config, "joint", "rightKey")
	}

	if leftKey == "" || rightKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			fmt.Sprintf("logit-scores: composite scale for %q requires leftKey and rightKey", stageKey),
			nil,
		))
	}

	leftScale, err := logitScores.compositeOperandScale(leftKey)

	if err != nil {
		return 0, err
	}

	rightScale, err := logitScores.compositeOperandScale(rightKey)

	if err != nil {
		return 0, err
	}

	if math.IsNaN(leftScale) || math.IsNaN(rightScale) ||
		math.IsInf(leftScale, 0) || math.IsInf(rightScale, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			fmt.Sprintf("logit-scores: composite scale operands for %q are non-finite", stageKey),
			nil,
		))
	}

	if leftScale > 0 && rightScale > 0 {
		return math.Sqrt(leftScale * rightScale), nil
	}

	return 0, errnie.Err(
		errnie.Validation,
		fmt.Sprintf("logit-scores: composite scale operands for %q are non-positive", stageKey),
		nil,
	)
}

func (logitScores *LogitScores) hasCenteredOperand(stageKey string) bool {
	leftKey, rightKey := logitScores.compositeOperandKeys(stageKey)

	return datura.Peek[string](logitScores.config, leftKey, "centerMode") == "median" ||
		datura.Peek[string](logitScores.config, rightKey, "centerMode") == "median"
}

func (logitScores *LogitScores) centeredCompositeScore(
	stageKey string,
	features map[string]float64,
	scales map[string]float64,
) (float64, error) {
	leftKey, rightKey := logitScores.compositeOperandKeys(stageKey)

	if leftKey == "" || rightKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			fmt.Sprintf("logit-scores: composite score for %q requires leftKey and rightKey", stageKey),
			nil,
		))
	}

	leftScore := normalizeFeature(features[leftKey], scales[leftKey])
	rightScore := normalizeFeature(features[rightKey], scales[rightKey])

	if math.IsNaN(leftScore) || math.IsNaN(rightScore) ||
		math.IsInf(leftScore, 0) || math.IsInf(rightScore, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			fmt.Sprintf("logit-scores: composite score operands for %q are non-finite", stageKey),
			nil,
		))
	}

	if leftScore <= 0 || rightScore <= 0 {
		return 0, nil
	}

	return math.Sqrt(leftScore * rightScore), nil
}

func (logitScores *LogitScores) compositeOperandKeys(stageKey string) (string, string) {
	leftKey := datura.Peek[string](logitScores.config, stageKey, "leftKey")
	rightKey := datura.Peek[string](logitScores.config, stageKey, "rightKey")

	if leftKey == "" || rightKey == "" {
		leftKey = datura.Peek[string](logitScores.config, "joint", "leftKey")
		rightKey = datura.Peek[string](logitScores.config, "joint", "rightKey")
	}

	return leftKey, rightKey
}

func (logitScores *LogitScores) compositeOperandScale(
	stageKey string,
) (float64, error) {
	return logitScores.priorMedianScale(stageKey)
}

func (logitScores *LogitScores) priorMedianScale(stageKey string) (float64, error) {
	samples := datura.Peek[[]float64](logitScores.config, "output", "scaleSamples", stageKey)

	if len(samples) > 0 {
		median, ok := statistic.MedianOf(samples)

		if ok && median > 0 && !math.IsNaN(median) && !math.IsInf(median, 0) {
			return median, nil
		}

		median, ok = positiveMedian(samples)

		if ok && median > 0 && !math.IsNaN(median) && !math.IsInf(median, 0) {
			return median, nil
		}
	}

	configured := datura.Peek[float64](logitScores.config, stageKey, "scale")

	if configured > 0 && !math.IsNaN(configured) && !math.IsInf(configured, 0) {
		return configured, nil
	}

	if len(samples) == 0 {
		return 0, errnie.Err(
			errnie.Validation,
			fmt.Sprintf("logit-scores: prior median scale for %q requires history", stageKey),
			nil,
		)
	}

	median, ok := statistic.MedianOf(samples)

	if !ok || median <= 0 || math.IsNaN(median) || math.IsInf(median, 0) {
		return 0, errnie.Err(
			errnie.Validation,
			fmt.Sprintf("logit-scores: prior median scale for %q could not be derived", stageKey),
			nil,
		)
	}

	return median, nil
}

func positiveMedian(samples []float64) (float64, bool) {
	positive := make([]float64, 0, len(samples))

	for _, sample := range samples {
		if sample > 0 {
			positive = append(positive, sample)
		}
	}

	return statistic.MedianOf(positive)
}

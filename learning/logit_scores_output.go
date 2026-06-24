package learning

import (
	"fmt"
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

func (logitScores *LogitScores) resolveOutputScore(
	state *datura.Artifact,
	outputKey string,
	computed float64,
	features map[string]float64,
	scales map[string]float64,
) (float64, error) {
	source := datura.Peek[string](logitScores.config, outputKey, "source")

	if source == "" {
		return computed, nil
	}

	rootKey := datura.Peek[string](logitScores.config, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"logit-scores: config root required",
			nil,
		))
	}

	var overrideValue float64
	overrideFound := false
	wireInputs := datura.Peek[[]string](state, "inputs")

	for wireIndex, wireInput := range wireInputs {
		if wireInput != source {
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

			overrideValue = features[wireIndex]
		}

		if rootKey != "features" {
			overrideValue = datura.Peek[float64](state, rootKey, wireInput)
		}

		overrideFound = true
	}

	if !overrideFound {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"logit-scores: source not in inputs",
			nil,
		))
	}

	combine := datura.Peek[string](logitScores.config, outputKey, "combine")

	if combine == "ratio" {
		if logitScores.hasCenteredOperand(outputKey) {
			return logitScores.centeredCompositeScore(outputKey, features, scales)
		}

		scale, err := logitScores.resolveCompositeScale(outputKey)

		if err != nil {
			if overrideValue == 0 {
				return 0, nil
			}

			return 0, err
		}

		if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
			if overrideValue == 0 {
				return 0, nil
			}

			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				fmt.Sprintf("logit-scores: output %q scale could not be derived", outputKey),
				nil,
			))
		}

		ratio := overrideValue / scale
		minRatio := datura.Peek[float64](logitScores.config, outputKey, "minRatio")

		if minRatio > 0 && ratio < minRatio {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				fmt.Sprintf(
					"logit-scores: output %q ratio %v below minRatio %v",
					outputKey, ratio, minRatio,
				),
				nil,
			))
		}

		return squashFeature(ratio), nil
	}

	scale, err := logitScores.resolveFeatureScale(outputKey, overrideValue, state)

	if err != nil {
		return 0, err
	}

	return normalizeFeature(overrideValue, scale), nil
}

func (logitScores *LogitScores) applyDecline(
	state *datura.Artifact,
	config *datura.Artifact,
	weights ClassifierWeights,
	features map[string]float64,
	scores []float64,
) error {
	declineSource := datura.Peek[string](config, "decline", "source")

	if declineSource == "" {
		return logitScores.applyGatedOutputs(state, config, scores)
	}

	declineValue, err := logitScores.readWireValue(state, declineSource)

	if err != nil {
		return err
	}

	declineNorm := squashFeature(declineValue)

	if datura.Peek[float64](config, "decline", "squash") <= 0 {
		declineNorm = declineValue
	}

	if declineNorm <= 0 {
		return logitScores.applyGatedOutputs(state, config, scores)
	}

	outputKey := datura.Peek[string](config, "decline", "output")

	if outputKey == "" {
		return errnie.Error(errnie.Err(
			errnie.Validation,
			"logit-scores: decline output required when decline source is configured",
			nil,
		))
	}

	index, err := outputIndex(config, outputKey)

	if err != nil {
		return err
	}

	scores[index] = declineNorm * weights.outputScore(outputKey, features)

	for _, attenuateKey := range datura.Peek[[]string](config, "decline", "attenuate") {
		attenuateIndex, err := outputIndex(config, attenuateKey)

		if err != nil {
			return err
		}

		scores[attenuateIndex] *= 1.0 - declineNorm
	}

	return logitScores.applyGatedOutputs(state, config, scores)
}

func (logitScores *LogitScores) applyGatedOutputs(
	state *datura.Artifact,
	config *datura.Artifact,
	scores []float64,
) error {
	outputs := datura.Peek[[]string](config, "outputs")

	for index, outputKey := range outputs {
		gateSource := datura.Peek[string](config, outputKey, "gate")

		if gateSource == "" {
			continue
		}

		gateValue, err := logitScores.readWireValue(state, gateSource)

		if err != nil {
			return err
		}

		gateActive := squashFeature(gateValue) <= 0

		if datura.Peek[float64](config, outputKey, "gateInvert") > 0 {
			gateActive = squashFeature(gateValue) > 0
		}

		if gateActive {
			scores[index] = 0
		}
	}

	return nil
}

func outputIndex(config *datura.Artifact, outputKey string) (int, error) {
	outputs := datura.Peek[[]string](config, "outputs")

	for index, key := range outputs {
		if key == outputKey {
			return index, nil
		}
	}

	return 0, errnie.Error(errnie.Err(
		errnie.Validation,
		fmt.Sprintf("logit-scores: output %q not listed in config outputs", outputKey),
		nil,
	))
}

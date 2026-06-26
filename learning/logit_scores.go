package learning

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
LogitScores maps configured feature outputs to classifier logits on the artifact.
Runtime scale and threshold history live on the config artifact output keys.
*/
type LogitScores struct {
	config *datura.Artifact
}

/*
NewLogitScores returns a classifier logit stage configured on the artifact.
*/
func NewLogitScores(config *datura.Artifact) *LogitScores {
	return &LogitScores{
		config: config,
	}
}

func (logitScores *LogitScores) Read(payload []byte) (int, error) {
	state := datura.Acquire("logit-scores-state", datura.APPJSON)

	if _, err := state.Unpack(logitScores.config.DecryptPayload()); err != nil {
		state.Release()

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"logit-scores: state write failed",
			err,
		))
	}

	defer state.Release()

	order := datura.Peek[[]string](logitScores.config, "order")
	outputs := datura.Peek[[]string](logitScores.config, "outputs")

	if len(order) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"logit-scores: config order required",
			nil,
		))
	}

	if len(outputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"logit-scores: config outputs required",
			nil,
		))
	}

	features, err := logitScores.featureValues(state, order)

	if err != nil {
		return 0, err
	}

	threshold, err := logitScores.resolveThreshold(features, state)

	if err != nil {
		return 0, err
	}

	scales, err := logitScores.featureScales(order, features, state)

	if err != nil {
		return 0, err
	}

	weightScales, suppressedScales, err := logitWeightScales(scales, features)

	if err != nil {
		return 0, err
	}

	weights, err := NewClassifierWeights(logitScores.config, threshold, weightScales)

	if err != nil {
		return 0, err
	}

	suppressZeroScaleTerms(&weights, suppressedScales)

	centeredFeatures := logitScores.centeredFeatures(features, scales)
	scores := weights.Scores(centeredFeatures)

	if err := logitScores.applyDecline(
		state, logitScores.config, weights, centeredFeatures, scores,
	); err != nil {
		return 0, err
	}

	for index, outputKey := range outputs {
		score, err := logitScores.resolveOutputScore(
			state, outputKey, scores[index], centeredFeatures, scales,
		)

		if err != nil {
			return 0, err
		}

		state.MergeOutput(outputKey, score)
	}

	state.MergeOutput("value", scores[0])

	strength := scores[0]

	for _, score := range scores[1:] {
		if score > strength {
			strength = score
		}
	}

	state.MergeOutput("strength", strength)
	state.Poke("output", "root")

	outputInputs := make([]string, 0, len(outputs)+1)
	outputInputs = append(outputInputs, outputs...)
	outputInputs = append(outputInputs, "strength")
	state.Poke(outputInputs, "inputs")

	return state.PackInto(payload)
}

func (logitScores *LogitScores) Write(payload []byte) (int, error) {
	logitScores.config.WithPayload(payload)
	return len(payload), nil
}

func (logitScores *LogitScores) Close() error {
	return nil
}

func logitWeightScales(
	scales map[string]float64,
	features map[string]float64,
) (map[string]float64, map[string]bool, error) {
	weightScales := make(map[string]float64, len(scales))
	suppressed := make(map[string]bool)

	for key, scale := range scales {
		feature := features[key]

		if scale == 0 && feature == 0 {
			// Placeholder scale only lets the strict constructor build; the term is suppressed before scoring.
			weightScales[key] = 1
			suppressed[key] = true
			continue
		}

		if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
			_, err := positiveScale(scale, key)
			return nil, nil, err
		}

		weightScales[key] = scale
	}

	return weightScales, suppressed, nil
}

func suppressZeroScaleTerms(weights *ClassifierWeights, suppressed map[string]bool) {
	if len(suppressed) == 0 {
		return
	}

	for outputKey, termWeights := range weights.termWeights {
		total := 0.0

		for featureKey, weight := range termWeights {
			if suppressed[featureKey] {
				termWeights[featureKey] = 0
				continue
			}

			total += weight
		}

		if total <= 0 {
			continue
		}

		for featureKey, weight := range termWeights {
			termWeights[featureKey] = weight / total
		}

		weights.termWeights[outputKey] = termWeights
	}
}

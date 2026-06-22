package learning

import (
	"fmt"
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/statistic"
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
	config.Inspect("learning", "logit-scores", "NewLogitScores()")

	return &LogitScores{
		config: config,
	}
}

func (logitScores *LogitScores) Write(payload []byte) (int, error) {
	logitScores.config.WithPayload(payload)
	return len(payload), nil
}

func (logitScores *LogitScores) Read(payload []byte) (int, error) {
	state := datura.Acquire("logit-scores-state", datura.APPJSON)
	state.Inspect("learning", "logit-scores", "Read()", "p")

	if _, err := state.Write(logitScores.config.DecryptPayload()); err != nil {
		state.Release()

		return 0, err
	}

	defer state.Release()

	order := configStringSlice(logitScores.config, state, "order")
	outputs := configStringSlice(logitScores.config, state, "outputs")

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

	threshold, err := logitScores.resolveThreshold(features)

	if err != nil {
		return 0, err
	}

	weights, err := logitScores.logitWeights(threshold, order, features)

	if err != nil {
		return 0, err
	}

	scores := weights.Scores(features)

	if err := logitScores.applyDecline(state, logitScores.config, weights, features, scores); err != nil {
		return 0, err
	}

	for index, outputKey := range outputs {
		score, err := logitScores.resolveOutputScore(state, outputKey, scores[index])

		if err != nil {
			return 0, err
		}

		state.MergeOutput(outputKey, score)
	}

	state.MergeOutput("value", scores[0])
	state.Merge("root", "output")
	state.Merge("inputs", outputs)

	return state.Read(payload)
}

func (logitScores *LogitScores) featureValues(
	state *datura.Artifact,
	order []string,
) (map[string]float64, error) {
	features := make(map[string]float64, len(order))

	for _, key := range order {
		wireKey := datura.Peek[string](logitScores.config, key, "source")

		if wireKey == "" {
			wireKey = key
		}

		value, err := logitScores.wireValue(state, wireKey)

		if err != nil {
			return nil, err
		}

		features[key] = value
	}

	return features, nil
}

func (logitScores *LogitScores) wireValue(
	state *datura.Artifact,
	wireKey string,
) (float64, error) {
	rootKey := configString(logitScores.config, state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"logit-scores: config root required",
			nil,
		))
	}

	inputKeys := configStringSlice(logitScores.config, state, "inputs")

	if len(inputKeys) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"logit-scores: config inputs required",
			nil,
		))
	}

	index := inputIndex(inputKeys, wireKey)

	if index < 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			fmt.Sprintf("logit-scores: wire key %q not listed in config inputs", wireKey),
			nil,
		))
	}

	if rootKey == "features" {
		features := datura.Peek[[]float64](state, "features")

		if index >= len(features) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				fmt.Sprintf("logit-scores: features index out of range for %q", wireKey),
				nil,
			))
		}

		return features[index], nil
	}

	return datura.Peek[float64](state, rootKey, wireKey), nil
}

func (logitScores *LogitScores) resolveOutputScore(
	state *datura.Artifact,
	outputKey string,
	computed float64,
) (float64, error) {
	source := datura.Peek[string](logitScores.config, outputKey, "source")

	if source == "" {
		return computed, nil
	}

	overrideValue, err := logitScores.wireValue(state, source)

	if err != nil {
		return 0, err
	}

	combine := datura.Peek[string](logitScores.config, outputKey, "combine")

	if combine == "ratio" {
		scale, err := logitScores.resolveFeatureScale(outputKey, overrideValue)

		if err != nil {
			return 0, err
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

		return ratio, nil
	}

	scale, err := logitScores.resolveFeatureScale(outputKey, overrideValue)

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

	declineValue, err := logitScores.wireValue(state, declineSource)

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

		gateValue, err := logitScores.wireValue(state, gateSource)

		if err != nil {
			return err
		}

		if squashFeature(gateValue) <= 0 {
			scores[index] = 0
		}
	}

	return nil
}

func inputIndex(inputKeys []string, wireKey string) int {
	for index, inputKey := range inputKeys {
		if inputKey == wireKey {
			return index
		}
	}

	return -1
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

func (logitScores *LogitScores) Close() error {
	return nil
}

func (logitScores *LogitScores) logitWeights(
	threshold float64,
	order []string,
	features map[string]float64,
) (ClassifierWeights, error) {
	scales := make(map[string]float64, len(order))

	for _, key := range order {
		scale, err := logitScores.resolveFeatureScale(key, features[key])

		if err != nil {
			return ClassifierWeights{}, err
		}

		scales[key] = scale
	}

	return NewClassifierWeights(logitScores.config, threshold, scales)
}

func (logitScores *LogitScores) resolveThreshold(
	features map[string]float64,
) (float64, error) {
	configured := datura.Peek[float64](logitScores.config, "threshold")

	if configured > 0 {
		return configured, nil
	}

	return logitScores.resolveThresholdFromFeatures(features)
}

func (logitScores *LogitScores) resolveThresholdFromFeatures(
	features map[string]float64,
) (float64, error) {
	magnitude := 0.0

	for _, feature := range features {
		magnitude += math.Abs(feature)
	}

	prior := datura.Peek[[]float64](logitScores.config, "output", "thresholdSamples")
	logitScores.config.Poke(append(prior, magnitude), "output", "thresholdSamples")

	if len(prior) < 2 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"logit-scores: insufficient threshold samples",
			nil,
		))
	}

	threshold, ok := statistic.MedianOf(prior)

	if !ok || threshold <= 0 || math.IsNaN(threshold) || math.IsInf(threshold, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"logit-scores: threshold could not be derived from features",
			nil,
		))
	}

	return threshold, nil
}

func (logitScores *LogitScores) resolveFeatureScale(
	stageKey string,
	feature float64,
) (float64, error) {
	configured := datura.Peek[float64](logitScores.config, stageKey, "scale")

	if configured > 0 {
		return configured, nil
	}

	samples := datura.Peek[[]float64](logitScores.config, "output", "scaleSamples", stageKey)
	derived := 0.0

	if len(samples) > 0 {
		median, ok := statistic.MedianOf(samples)

		if ok {
			derived = median
		}
	}

	samples = append(samples, math.Abs(feature))
	logitScores.config.Poke(samples, "output", "scaleSamples", stageKey)

	scale := math.Max(derived, math.Abs(feature))
	scaleMode := datura.Peek[string](logitScores.config, stageKey, "scaleMode")

	if scaleMode == "median" && derived > 0 {
		scale = derived
	}

	if scaleMode == "median" && derived <= 0 {
		composite, err := logitScores.resolveCompositeScale(stageKey)

		if err != nil {
			return 0, err
		}

		scale = composite
	}

	if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			fmt.Sprintf("logit-scores: scale for %q could not be derived", stageKey),
			nil,
		))
	}

	return scale, nil
}

func (logitScores *LogitScores) resolveCompositeScale(stageKey string) (float64, error) {
	leftKey := datura.Peek[string](logitScores.config, stageKey, "leftKey")
	rightKey := datura.Peek[string](logitScores.config, stageKey, "rightKey")

	if leftKey == "" || rightKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			fmt.Sprintf("logit-scores: composite scale for %q requires leftKey and rightKey", stageKey),
			nil,
		))
	}

	leftScale, err := logitScores.priorMedianScale(leftKey)

	if err != nil {
		return 0, err
	}

	rightScale, err := logitScores.priorMedianScale(rightKey)

	if err != nil {
		return 0, err
	}

	return math.Sqrt(leftScale * rightScale), nil
}

func (logitScores *LogitScores) priorMedianScale(stageKey string) (float64, error) {
	samples := datura.Peek[[]float64](logitScores.config, "output", "scaleSamples", stageKey)

	if len(samples) < 2 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			fmt.Sprintf("logit-scores: prior median scale for %q requires history", stageKey),
			nil,
		))
	}

	median, ok := statistic.MedianOf(samples[:len(samples)-1])

	if !ok {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			fmt.Sprintf("logit-scores: prior median scale for %q could not be derived", stageKey),
			nil,
		))
	}

	return median, nil
}

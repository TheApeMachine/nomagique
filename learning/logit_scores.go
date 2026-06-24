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
	return &LogitScores{
		config: config,
	}
}

func (logitScores *LogitScores) Read(payload []byte) (int, error) {
	state := datura.Acquire("logit-scores-state", datura.APPJSON)

	if _, err := state.Write(logitScores.config.DecryptPayload()); err != nil {
		state.Release()

		return 0, err
	}

	state.Inspect("learning", "logit-scores", "Read()", "p")

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

	weights, err := NewClassifierWeights(logitScores.config, threshold, scales)

	if err != nil {
		return 0, err
	}

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

	return state.Read(payload)
}

func (logitScores *LogitScores) featureValues(
	state *datura.Artifact,
	order []string,
) (map[string]float64, error) {
	scoreRoot := datura.Peek[string](logitScores.config, "scoreRoot")

	if scoreRoot != "" {
		return logitScores.featureValuesFromScoreRoot(state, order, scoreRoot)
	}

	features := make(map[string]float64, len(order))

	for _, key := range order {
		wireKey := datura.Peek[string](logitScores.config, key, "source")

		if wireKey == "" {
			wireKey = key
		}

		rootKey := datura.Peek[string](logitScores.config, "root")

		if rootKey == "" {
			return nil, errnie.Error(errnie.Err(
				errnie.Validation,
				"logit-scores: config root required",
				nil,
			))
		}

		var value float64
		valueFound := false
		wireInputs := datura.Peek[[]string](state, "inputs")

		for wireIndex, wireInput := range wireInputs {
			if wireInput != wireKey {
				continue
			}

			if rootKey == "features" {
				features := datura.Peek[[]float64](state, rootKey)

				if wireIndex >= len(features) {
					return nil, errnie.Error(errnie.Err(
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
			return nil, errnie.Error(errnie.Err(
				errnie.Validation,
				"logit-scores: wire key not in inputs",
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
	scoreRoot := datura.Peek[string](logitScores.config, "scoreRoot")

	if scoreRoot != "" {
		outputMap := datura.Peek[map[string]any](state, scoreRoot)

		if _, ok := outputMap[wireKey]; !ok {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"logit-scores: score key missing from "+scoreRoot,
				nil,
			))
		}

		value := datura.Peek[float64](state, scoreRoot, wireKey)

		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"logit-scores: score is non-finite",
				nil,
			))
		}

		return value, nil
	}

	rootKey := datura.Peek[string](logitScores.config, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"logit-scores: config root required",
			nil,
		))
	}

	wireInputs := datura.Peek[[]string](state, "inputs")

	for wireIndex, wireInput := range wireInputs {
		if wireInput != wireKey {
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

			return features[wireIndex], nil
		}

		value := datura.Peek[float64](state, rootKey, wireInput)

		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"logit-scores: score is non-finite",
				nil,
			))
		}

		return value, nil
	}

	return 0, errnie.Error(errnie.Err(
		errnie.Validation,
		"logit-scores: wire key not in inputs",
		nil,
	))
}

func (logitScores *LogitScores) featureValuesFromScoreRoot(
	state *datura.Artifact,
	order []string,
	scoreRoot string,
) (map[string]float64, error) {
	features := make(map[string]float64, len(order))

	for _, key := range order {
		wireKey := datura.Peek[string](logitScores.config, key, "source")

		if wireKey == "" {
			wireKey = key
		}

		value := datura.Peek[float64](state, scoreRoot, wireKey)
		outputMap := datura.Peek[map[string]any](state, scoreRoot)

		if _, ok := outputMap[wireKey]; !ok {
			return nil, errnie.Error(errnie.Err(
				errnie.Validation,
				"logit-scores: score key missing from "+scoreRoot,
				nil,
			))
		}

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

func (logitScores *LogitScores) Write(payload []byte) (int, error) {
	logitScores.config.WithPayload(payload)
	return len(payload), nil
}

func (logitScores *LogitScores) Close() error {
	return nil
}

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

func (logitScores *LogitScores) resolveFeatureScale(
	stageKey string,
	feature float64,
	state *datura.Artifact,
) (float64, error) {
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

	if derived <= 0 && feature > 0 {
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

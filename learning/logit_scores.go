package learning

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/statistic"
)

/*
LogitScores maps configured feature outputs to classifier logits on the artifact.
Runtime scale and threshold history live on the stage instance, not the config artifact.
*/
type LogitScores struct {
	config           *datura.Artifact
	scaleSamples     map[string][]float64
	thresholdSamples []float64
}

/*
NewLogitScores returns a classifier logit stage configured on the artifact.
*/
func NewLogitScores(config *datura.Artifact) *LogitScores {
	config.Inspect("learning", "logit-scores", "NewLogitScores()")

	return &LogitScores{
		config:       config,
		scaleSamples: map[string][]float64{},
	}
}

func (logitScores *LogitScores) Write(p []byte) (int, error) {
	logitScores.config.WithPayload(p)

	return len(p), nil
}

func (logitScores *LogitScores) Read(payload []byte) (int, error) {
	state := datura.Acquire("logit-scores-state", datura.APPJSON)
	state.Inspect("learning", "logit-scores", "Read()", "p")

	if _, err := state.Write(logitScores.config.DecryptPayload()); err != nil {
		state.Release()

		return 0, err
	}

	defer state.Release()

	order := datura.Peek[[]string](logitScores.config, "order")
	outputs := datura.Peek[[]string](logitScores.config, "outputs")

	if len(order) == 0 || len(outputs) == 0 {
		return state.Read(payload)
	}

	features := logitScores.featureValues(state, order)
	threshold := logitScores.resolveThreshold(features)

	if threshold <= 0 {
		return state.Read(payload)
	}

	weights, err := logitScores.logitWeights(threshold, order, features)

	if err != nil {
		return state.Read(payload)
	}

	scores := weights.Scores(features)
	logitScores.applyDecline(state, logitScores.config, weights, features, scores)

	for index, outputKey := range outputs {
		score := scores[index]
		score = logitScores.resolveOutputScore(state, outputKey, score)

		state.MergeOutput(outputKey, score)
	}

	if len(scores) > 0 {
		state.MergeOutput("value", scores[0])
	}

	state.Merge("root", "output")
	state.Merge("inputs", outputs)

	return state.Read(payload)
}

func (logitScores *LogitScores) featureValues(
	state *datura.Artifact,
	order []string,
) map[string]float64 {
	features := make(map[string]float64, len(order))

	for _, key := range order {
		features[key] = logitScores.featureValue(state, key)
	}

	return features
}

func (logitScores *LogitScores) featureValue(state *datura.Artifact, key string) float64 {
	source := datura.Peek[string](logitScores.config, "inputs", key, "source")

	if source == "" {
		source = key
	}

	return datura.Peek[float64](state, "output", source)
}

func (logitScores *LogitScores) resolveOutputScore(
	state *datura.Artifact,
	outputKey string,
	fallback float64,
) float64 {
	source := datura.Peek[string](logitScores.config, "inputs", outputKey, "source")

	if source == "" {
		source = datura.Peek[string](logitScores.config, "inputs", "joint", "source")

		if datura.Peek[string](logitScores.config, "inputs", "joint", "output") != outputKey {
			source = ""
		}
	}

	if source == "" {
		return fallback
	}

	overrideValue := datura.Peek[float64](state, "output", source)

	if overrideValue <= 0 {
		return fallback
	}

	combine := datura.Peek[string](logitScores.config, "inputs", outputKey, "combine")

	if combine == "" {
		combine = datura.Peek[string](logitScores.config, "inputs", "joint", "combine")
	}

	if combine == "ratio" {
		scaleKey := outputKey
		jointOutput := datura.Peek[string](logitScores.config, "inputs", "joint", "output")

		if jointOutput != "" && jointOutput == outputKey {
			scaleKey = "joint"
		}

		scale := logitScores.resolveFeatureScale(scaleKey, overrideValue)

		if scale > 0 {
			ratio := overrideValue / scale
			minRatio := datura.Peek[float64](logitScores.config, "inputs", outputKey, "minRatio")

			if minRatio <= 0 {
				minRatio = datura.Peek[float64](logitScores.config, "inputs", "joint", "minRatio")
			}

			if minRatio > 0 && ratio < minRatio {
				return fallback
			}

			return ratio
		}
	}

	scale := logitScores.resolveFeatureScale(outputKey, overrideValue)

	return normalizeFeature(overrideValue, scale)
}

func (logitScores *LogitScores) applyDecline(
	state *datura.Artifact,
	config *datura.Artifact,
	weights ClassifierWeights,
	features map[string]float64,
	scores []float64,
) {
	declineSource := datura.Peek[string](config, "inputs", "decline", "source")

	if declineSource == "" {
		return
	}

	declineValue := datura.Peek[float64](state, "output", declineSource)

	if declineValue <= 0 {
		declineValue = datura.Peek[float64](state, declineSource)
	}

	if declineValue <= 0 {
		declineValue = datura.Peek[float64](config, "state", declineSource)
	}

	if declineValue <= 0 {
		declineValue = datura.Peek[float64](config, declineSource)
	}

	declineNorm := squashFeature(declineValue)

	if datura.Peek[float64](config, "inputs", "decline", "squash") <= 0 {
		declineNorm = declineValue
	}

	if declineNorm <= 0 {
		logitScores.applyGatedOutputs(state, config, scores)

		return
	}

	outputKey := datura.Peek[string](config, "inputs", "decline", "output")

	if outputKey != "" {
		index := outputIndex(config, outputKey)

		if index >= 0 {
			scores[index] = declineNorm * weights.outputScore(outputKey, features)
		}
	}

	for _, attenuateKey := range datura.Peek[[]string](config, "inputs", "decline", "attenuate") {
		index := outputIndex(config, attenuateKey)

		if index >= 0 {
			scores[index] *= 1.0 - declineNorm
		}
	}

	logitScores.applyGatedOutputs(state, config, scores)
}

func (logitScores *LogitScores) applyGatedOutputs(
	state *datura.Artifact,
	config *datura.Artifact,
	scores []float64,
) {
	outputs := datura.Peek[[]string](config, "outputs")

	for index, outputKey := range outputs {
		gateSource := datura.Peek[string](config, "inputs", outputKey, "gate")

		if gateSource == "" {
			continue
		}

		gateValue := datura.Peek[float64](state, "output", gateSource)

		if gateValue <= 0 {
			gateValue = datura.Peek[float64](state, gateSource)
		}

		if gateValue <= 0 {
			gateValue = datura.Peek[float64](config, "state", gateSource)
		}

		if gateValue <= 0 {
			gateValue = datura.Peek[float64](config, gateSource)
		}

		if squashFeature(gateValue) <= 0 {
			scores[index] = 0
		}
	}
}

func outputIndex(config *datura.Artifact, outputKey string) int {
	outputs := datura.Peek[[]string](config, "outputs")

	for index, key := range outputs {
		if key == outputKey {
			return index
		}
	}

	return -1
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
		scales[key] = logitScores.resolveFeatureScale(key, features[key])
	}

	return NewClassifierWeights(logitScores.config, threshold, scales)
}

func (logitScores *LogitScores) resolveThreshold(features map[string]float64) float64 {
	configured := datura.Peek[float64](logitScores.config, "threshold")

	if configured > 0 {
		return configured
	}

	return logitScores.resolveThresholdFromFeatures(features)
}

func (logitScores *LogitScores) resolveThresholdFromFeatures(features map[string]float64) float64 {
	magnitude := 0.0

	for _, feature := range features {
		magnitude += math.Abs(feature)
	}

	derived := 0.0

	if len(logitScores.thresholdSamples) > 0 {
		derived = statistic.MedianOf(logitScores.thresholdSamples)
	}

	logitScores.thresholdSamples = append(logitScores.thresholdSamples, magnitude)

	threshold := math.Max(derived, magnitude)

	if threshold <= 0 || math.IsNaN(threshold) || math.IsInf(threshold, 0) {
		return math.SmallestNonzeroFloat64
	}

	return threshold
}

func (logitScores *LogitScores) resolveFeatureScale(stageKey string, feature float64) float64 {
	configured := datura.Peek[float64](logitScores.config, "inputs", stageKey, "scale")

	if configured > 0 {
		return configured
	}

	samples := logitScores.scaleSamples[stageKey]
	derived := 0.0

	if len(samples) > 0 {
		derived = statistic.MedianOf(samples)
	}

	samples = append(samples, math.Abs(feature))
	logitScores.scaleSamples[stageKey] = samples

	scale := math.Max(derived, math.Abs(feature))

	scaleMode := datura.Peek[string](logitScores.config, "inputs", stageKey, "scaleMode")

	if scaleMode == "median" && derived > 0 {
		scale = derived
	}

	if scaleMode == "median" && derived <= 0 {
		composite := logitScores.resolveCompositeScale(stageKey)

		if composite > 0 {
			scale = composite
		}
	}

	if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		scale = logitScores.peerScaleFloor(stageKey)
	}

	return scale
}

func (logitScores *LogitScores) resolveCompositeScale(stageKey string) float64 {
	leftKey := datura.Peek[string](logitScores.config, "inputs", stageKey, "leftKey")
	rightKey := datura.Peek[string](logitScores.config, "inputs", stageKey, "rightKey")

	if leftKey == "" || rightKey == "" {
		return 0
	}

	leftScale := logitScores.priorMedianScale(leftKey)
	rightScale := logitScores.priorMedianScale(rightKey)

	if leftScale <= 0 {
		return 0
	}

	if rightScale <= 0 {
		return leftScale
	}

	return math.Sqrt(leftScale * rightScale)
}

func (logitScores *LogitScores) priorMedianScale(stageKey string) float64 {
	samples := logitScores.scaleSamples[stageKey]

	if len(samples) == 0 {
		return 0
	}

	if len(samples) == 1 {
		return samples[0]
	}

	return statistic.MedianOf(samples[:len(samples)-1])
}

func (logitScores *LogitScores) peerScaleFloor(stageKey string) float64 {
	order := datura.Peek[[]string](logitScores.config, "order")

	if len(order) == 0 {
		return math.SmallestNonzeroFloat64
	}

	peer := 0.0

	for _, key := range order {
		if key == stageKey {
			continue
		}

		peerSamples := logitScores.scaleSamples[key]

		if len(peerSamples) == 0 {
			continue
		}

		median := statistic.MedianOf(peerSamples)

		if median > peer {
			peer = median
		}
	}

	if peer <= 0 || math.IsNaN(peer) || math.IsInf(peer, 0) {
		return math.SmallestNonzeroFloat64
	}

	return peer / float64(len(order))
}

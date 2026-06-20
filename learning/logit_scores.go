package learning

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/statistic"
)

/*
LogitScores maps configured feature outputs to classifier logits on the artifact.
*/
type LogitScores struct {
	config *datura.Artifact
	bytes  []byte
}

/*
NewLogitScores returns a classifier logit stage configured on the artifact.
*/
func NewLogitScores(config *datura.Artifact) *LogitScores {
	return &LogitScores{
		config: config,
	}
}

func (logitScores *LogitScores) Write(p []byte) (int, error) {
	logitScores.bytes = append(logitScores.bytes[:0], p...)

	return len(p), nil
}

func (logitScores *LogitScores) Read(payload []byte) (int, error) {
	state := datura.Acquire("logit-scores-state", datura.APPJSON)

	if _, err := state.Write(logitScores.bytes); err != nil {
		state.Release()

		return 0, err
	}

	defer state.Release()

	order := datura.Peek[[]string](logitScores.config, "order")
	outputs := datura.Peek[[]string](logitScores.config, "outputs")

	if len(order) < 3 || len(outputs) < 4 {
		return state.Read(payload)
	}

	features := make([]float64, 3)

	for index, key := range order[:3] {
		features[index] = logitScores.featureValue(state, key)
	}

	threshold := logitScores.resolveThreshold(features)

	if threshold <= 0 {
		return state.Read(payload)
	}

	weights := logitWeights(logitScores.config, order[:3], features)
	scales := weights.FeatureScales()
	scores := weights.Scores(features[0], features[1], features[2])

	rvolDecline := datura.Peek[float64](logitScores.config, "rvolDecline")
	precursorNorm := normalizeFeature(features[1], scales.Precursor)
	declineNorm := squashFeature(rvolDecline)

	if declineNorm > 0 {
		scores[3] = declineNorm * (weights.WExVol + (1.0-precursorNorm)*weights.WExPrec)
		scores[1] *= 1.0 - declineNorm
	} else {
		scores[3] = 0
	}
	overrideValue, overrideOutput := logitScores.jointOverride(state)

	output := datura.Acquire("logit-scores-output", datura.APPJSON)
	output.WithPayload(state.DecryptPayload())

	for index, outputKey := range outputs[:4] {
		score := scores[index]

		if outputKey == overrideOutput && overrideValue > 0 {
			jointNorm := squashFeature(
				overrideValue / math.Sqrt(scales.RVol*scales.Precursor),
			)

			if jointNorm > 0 {
				score = jointNorm
			}
		}

		output.MergeOutput(outputKey, score)
	}

	output.MergeOutput("value", scores[0])

	return output.Read(payload)
}

func (logitScores *LogitScores) featureValue(state *datura.Artifact, key string) float64 {
	source := datura.Peek[string](logitScores.config, "inputs", key, "source")

	if source == "" {
		source = key
	}

	return datura.Peek[float64](state, "output", source)
}

func (logitScores *LogitScores) jointOverride(state *datura.Artifact) (float64, string) {
	source := datura.Peek[string](logitScores.config, "inputs", "joint", "source")
	output := datura.Peek[string](logitScores.config, "inputs", "joint", "output")

	if source == "" {
		return 0, output
	}

	return datura.Peek[float64](state, "output", source), output
}

func (logitScores *LogitScores) Close() error {
	return nil
}

func logitWeights(config *datura.Artifact, order []string, features []float64) ClassifierWeights {
	threshold := datura.Peek[float64](config, "threshold")

	if threshold <= 0 {
		threshold = resolveThresholdFromFeatures(config, features)
	}

	scales := ClassifierFeatureScales{
		RVol:        resolveFeatureScale(config, order[0], features[0]),
		Precursor:   resolveFeatureScale(config, order[1], features[1]),
		Compression: resolveFeatureScale(config, order[2], features[2]),
	}

	weights, err := NewClassifierWeights(threshold, scales)

	if err != nil {
		return ClassifierWeights{Threshold: threshold}
	}

	return weights
}

func (logitScores *LogitScores) resolveThreshold(features []float64) float64 {
	configured := datura.Peek[float64](logitScores.config, "threshold")

	if configured > 0 {
		return configured
	}

	return resolveThresholdFromFeatures(logitScores.config, features)
}

func resolveThresholdFromFeatures(config *datura.Artifact, features []float64) float64 {
	magnitude := 0.0

	for _, feature := range features {
		magnitude += math.Abs(feature)
	}

	samples := datura.Peek[[]float64](config, "thresholdSamples")
	derived := 0.0

	if len(samples) > 0 {
		derived = statistic.MedianOf(samples)
	}

	samples = append(samples, magnitude)
	config.Merge("thresholdSamples", samples)

	threshold := math.Max(derived, magnitude)

	if threshold <= 0 || math.IsNaN(threshold) || math.IsInf(threshold, 0) {
		return math.SmallestNonzeroFloat64
	}

	return threshold
}

func resolveFeatureScale(config *datura.Artifact, stageKey string, feature float64) float64 {
	configured := datura.Peek[float64](config, "inputs", stageKey, "scale")

	if configured > 0 {
		return configured
	}

	historyKey := "scaleSamples." + stageKey
	samples := datura.Peek[[]float64](config, historyKey)
	derived := 0.0

	if len(samples) > 0 {
		derived = statistic.MedianOf(samples)
	}

	samples = append(samples, math.Abs(feature))
	config.Merge(historyKey, samples)

	scale := math.Max(derived, math.Abs(feature))

	if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		scale = peerScaleFloor(config, stageKey)
	}

	return scale
}

func peerScaleFloor(config *datura.Artifact, stageKey string) float64 {
	order := datura.Peek[[]string](config, "order")

	if len(order) == 0 {
		return math.SmallestNonzeroFloat64
	}

	peer := 0.0

	for _, key := range order {
		if key == stageKey {
			continue
		}

		peerSamples := datura.Peek[[]float64](config, "scaleSamples."+key)

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

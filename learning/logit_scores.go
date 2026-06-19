package learning

import (
	"github.com/theapemachine/datura"
)

/*
LogitScores maps configured feature outputs to classifier logits on the artifact.
*/
type LogitScores struct {
	config *datura.Artifact
	staged *datura.Artifact
}

/*
NewLogitScores returns a classifier logit stage configured on the artifact.
*/
func NewLogitScores(config *datura.Artifact) *LogitScores {
	return &LogitScores{
		config: config,
		staged: datura.Acquire("logit-scores", datura.APPJSON),
	}
}

func (logitScores *LogitScores) Write(payload []byte) (int, error) {
	return logitScores.staged.Write(payload)
}

func (logitScores *LogitScores) Read(payload []byte) (int, error) {
	order := datura.Peek[[]string](logitScores.config, "order")
	outputs := datura.Peek[[]string](logitScores.config, "outputs")

	threshold := datura.Peek[float64](logitScores.config, "threshold")

	if len(order) < 3 || len(outputs) < 4 || threshold <= 0 {
		return logitScores.staged.Read(payload)
	}

	features := make([]float64, 3)

	for index, key := range order[:3] {
		features[index] = logitScores.featureValue(key)
	}

	weights := logitWeights(logitScores.config, order[:3])
	scores := weights.Scores(features[0], features[1], features[2])
	overrideValue, overrideOutput := logitScores.jointOverride()

	for index, outputKey := range outputs[:4] {
		score := scores[index]

		if outputKey == overrideOutput && overrideValue > 0 {
			score = overrideValue
		}

		logitScores.staged.Poke(score, "output", outputKey)
	}

	logitScores.staged.Poke(scores[0], "output", "value")

	return logitScores.staged.Read(payload)
}

func (logitScores *LogitScores) featureValue(key string) float64 {
	source := datura.Peek[string](logitScores.config, "inputs", key, "source")

	if source == "" {
		source = key
	}

	return datura.Peek[float64](logitScores.staged, "output", source)
}

func (logitScores *LogitScores) jointOverride() (float64, string) {
	source := datura.Peek[string](logitScores.config, "inputs", "joint", "source")
	output := datura.Peek[string](logitScores.config, "inputs", "joint", "output")

	if source == "" {
		return 0, output
	}

	return datura.Peek[float64](logitScores.staged, "output", source), output
}

func (logitScores *LogitScores) Close() error {
	return nil
}

func logitWeights(config *datura.Artifact, order []string) ClassifierWeights {
	threshold := datura.Peek[float64](config, "threshold")

	scales := ClassifierFeatureScales{
		RVol:        datura.Peek[float64](config, "inputs", order[0], "scale"),
		Precursor:   datura.Peek[float64](config, "inputs", order[1], "scale"),
		Compression: datura.Peek[float64](config, "inputs", order[2], "scale"),
	}

	weights, err := NewClassifierWeights(threshold, scales)

	if err != nil {
		return ClassifierWeights{Threshold: threshold}
	}

	return weights
}

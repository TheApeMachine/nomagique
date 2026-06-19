package probability

import (
	"io"
	"math"

	"github.com/theapemachine/datura"
)

/*
Classifier selects a category from competing scores declared on inputs.

Each input names a key under output on the carried artifact payload. Upstream stages
poke those scores before Classifier.Read runs.
*/
type Classifier struct {
	artifact *datura.Artifact
}

/*
NewClassifier returns a classifier wired from schema.inputs score keys.
*/
func NewClassifier(artifact *datura.Artifact) *Classifier {
	return &Classifier{artifact: artifact}
}

func (classifier *Classifier) Write(p []byte) (int, error) {
	inbound := datura.Acquire("classifier-inbound", datura.APPJSON)
	_, _ = inbound.Write(p)

	inputs := datura.Peek[[]string](classifier.artifact, "inputs")

	if len(inputs) == 0 {
		inputs = datura.Peek[[]string](inbound, "inputs")
	}

	scores := make(map[string]float64, len(inputs))

	for _, input := range inputs {
		scores[input] = datura.Peek[float64](inbound, "output", input)
	}

	n, err := classifier.artifact.Write(p)

	if len(inputs) > 0 {
		classifier.artifact.Poke(inputs, "inputs")
	}

	for key, score := range scores {
		if score != 0 {
			classifier.artifact.Poke(score, "output", key)
		}
	}

	return n, err
}

func (classifier *Classifier) Read(p []byte) (int, error) {
	inputs := datura.Peek[[]string](classifier.artifact, "inputs")

	if len(inputs) == 0 {
		return classifier.artifact.Read(p)
	}

	scores := make([]float64, len(inputs))

	for index, input := range inputs {
		if input == "" {
			return classifier.artifact.Read(p)
		}

		score := datura.Peek[float64](classifier.artifact, "output", input)

		if math.IsNaN(score) || math.IsInf(score, 0) {
			return classifier.artifact.Read(p)
		}

		scores[index] = score
	}

	probabilities, err := SoftmaxScoresNormalized(scores)

	if err != nil {
		return classifier.artifact.Read(p)
	}

	categoryIndex := ArgmaxIndex(probabilities) + 1

	confidence, confidenceErr := CategoryConfidence(probabilities, categoryIndex)

	if confidenceErr != nil {
		return classifier.artifact.Read(p)
	}

	strength := datura.Peek[float64](classifier.artifact, "output", "strength")

	if strength <= 0 || math.IsNaN(strength) || math.IsInf(strength, 0) {
		strength = scores[categoryIndex-1]
	}

	classifier.artifact.Poke(probabilities, "output", "probabilities")
	classifier.artifact.Poke(float64(categoryIndex), "output", "category")
	classifier.artifact.Poke(confidence, "output", "confidence")
	classifier.artifact.Poke(strength, "output", "strength")
	classifier.artifact.Poke(float64(categoryIndex), "output", "value")

	return classifier.artifact.Read(p)
}

func (classifier *Classifier) Close() error {
	return nil
}

var _ io.ReadWriteCloser = (*Classifier)(nil)

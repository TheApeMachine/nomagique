package probability

import (
	"io"
	"math"

	"github.com/theapemachine/datura"
)

/*
Classifier selects a category from competing scores declared on inputs.

Each input names a key under output on the carried artifact. Upstream stages
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
	inputs := datura.Peek[[]string](classifier.artifact, "inputs")

	n, err := classifier.artifact.Write(p)

	if len(inputs) > 0 {
		classifier.artifact.Poke(inputs, "inputs")
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

	classifier.artifact.Poke(probabilities, "classifier", "probabilities")
	classifier.artifact.Poke(categoryIndex, "classifier", "category")
	classifier.artifact.Poke(confidence, "classifier", "confidence")
	classifier.artifact.Poke(float64(categoryIndex), "output", "value")

	return classifier.artifact.Read(p)
}

func (classifier *Classifier) Close() error {
	return nil
}

var _ io.ReadWriter = (*Classifier)(nil)

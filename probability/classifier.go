package probability

import (
	"io"
	"math"

	"github.com/theapemachine/datura"
)

/*
Classifier selects a category from competing scores declared on inputs.

The config artifact carries input score keys in attributes; its payload buses
inbound wire from Write to Read. Read classifies from reconstituted output scores.
*/
type Classifier struct {
	config *datura.Artifact
}

/*
NewClassifier returns a classifier wired from schema.inputs score keys.
*/
func NewClassifier(config *datura.Artifact) *Classifier {
	return &Classifier{config: config}
}

func (classifier *Classifier) Write(p []byte) (int, error) {
	classifier.config.WithPayload(p)
	return len(p), nil
}

func (classifier *Classifier) Read(payload []byte) (int, error) {
	state := datura.Acquire("classifier-state", datura.APPJSON)

	if _, err := state.Write(classifier.config.DecryptPayload()); err != nil {
		state.Release()
		return 0, err
	}

	defer state.Release()

	inputs := datura.Peek[[]string](classifier.config, "inputs")

	if len(inputs) == 0 {
		inputs = datura.Peek[[]string](state, "inputs")
	}

	if len(inputs) == 0 {
		return state.Read(payload)
	}

	scores := make([]float64, len(inputs))

	for index, input := range inputs {
		if input == "" {
			return state.Read(payload)
		}

		score := datura.Peek[float64](state, "output", input)

		if math.IsNaN(score) || math.IsInf(score, 0) {
			return state.Read(payload)
		}

		scores[index] = score
	}

	probabilities, err := SoftmaxScoresNormalized(scores)

	if err != nil {
		return state.Read(payload)
	}

	categoryIndex := ArgmaxIndex(probabilities) + 1

	confidence, confidenceErr := CategoryConfidence(probabilities, categoryIndex)

	if confidenceErr != nil {
		return state.Read(payload)
	}

	strength := datura.Peek[float64](state, "output", "strength")

	if strength <= 0 || math.IsNaN(strength) || math.IsInf(strength, 0) {
		strength = scores[categoryIndex-1]
	}

	state.MergeOutput("probabilities", probabilities)
	state.MergeOutput("category", categoryIndex)
	state.MergeOutput("confidence", confidence)
	state.MergeOutput("strength", strength)
	state.MergeOutput("value", categoryIndex)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})

	return state.Read(payload)
}

func (classifier *Classifier) Close() error {
	return nil
}

var _ io.ReadWriteCloser = (*Classifier)(nil)

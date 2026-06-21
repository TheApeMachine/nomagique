package probability

import (
	"io"
	"math"

	"github.com/theapemachine/datura"
)

/*
Classifier selects a category from competing scores declared on inputs.

The config artifact carries the input score keys; the inbound artifact wire is
buffered per frame. Read reconstitutes that state, classifies from payload
output scores, and writes the result back onto the payload.
*/
type Classifier struct {
	config *datura.Artifact
	bytes  []byte
}

/*
NewClassifier returns a classifier wired from schema.inputs score keys.
*/
func NewClassifier(config *datura.Artifact) *Classifier {
	config.Inspect("probability", "classifier", "NewClassifier()")

	return &Classifier{config: config}
}

func (classifier *Classifier) Write(payload []byte) (int, error) {
	classifier.bytes = append(classifier.bytes[:0], payload...)

	return len(payload), nil
}

func (classifier *Classifier) Read(payload []byte) (int, error) {
	state := datura.Acquire("classifier-state", datura.APPJSON)
	state.Inspect("probability", "classifier", "Read()", "p")

	if _, err := state.Write(classifier.bytes); err != nil {
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

	output := datura.Acquire("classifier-output", datura.APPJSON)
	output.WithPayload(state.DecryptPayload())
	output.MergeOutput("probabilities", probabilities)
	output.MergeOutput("category", categoryIndex)
	output.MergeOutput("confidence", confidence)
	output.MergeOutput("strength", strength)
	output.MergeOutput("value", categoryIndex)

	output.Inspect("probability", "classifier", "Read()", "output")

	return output.Read(payload)
}

func (classifier *Classifier) Close() error {
	return nil
}

var _ io.ReadWriteCloser = (*Classifier)(nil)

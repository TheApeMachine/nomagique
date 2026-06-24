package probability

import (
	"io"
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
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

func (classifier *Classifier) Read(payload []byte) (int, error) {
	state := datura.Acquire("classifier-state", datura.APPJSON)

	if _, err := state.Write(classifier.config.DecryptPayload()); err != nil {
		state.Release()
		return 0, err
	}

	state.Inspect("probability", "classifier", "Read()", "p")

	defer state.Release()

	inputs := datura.Peek[[]string](classifier.config, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"classifier: inputs required",
			nil,
		))
	}

	scoreRoot := datura.Peek[string](classifier.config, "scoreRoot")

	if scoreRoot == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"classifier: scoreRoot required",
			nil,
		))
	}

	scores := make([]float64, len(inputs))

	for index, input := range inputs {
		if input == "" {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"classifier: empty input key",
				nil,
			))
		}

		var score float64
		scoreFound := false

		for wireIndex, wireInput := range datura.Peek[[]string](state, "inputs") {
			if wireInput != input {
				continue
			}

			if scoreRoot == "features" {
				features := datura.Peek[[]float64](state, scoreRoot)

				if wireIndex >= len(features) {
					return 0, errnie.Error(errnie.Err(
						errnie.Validation,
						"classifier: feature index out of range",
						nil,
					))
				}

				score = features[wireIndex]
			}

			if scoreRoot != "features" {
				score = datura.Peek[float64](state, scoreRoot, wireInput)
			}

			scoreFound = true
		}

		if !scoreFound {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"classifier: input not in inputs",
				nil,
			))
		}

		if math.IsNaN(score) || math.IsInf(score, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"classifier: score is non-finite",
				nil,
			))
		}

		scores[index] = score
	}

	allZeroEvidence := true

	for _, score := range scores {
		if score > 0 {
			allZeroEvidence = false

			break
		}
	}

	probabilities, err := SoftmaxScoresNormalized(scores)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"classifier: unable to compute softmax probabilities",
			err,
		))
	}

	categoryIndex := ArgmaxIndex(probabilities) + 1

	confidence, confidenceErr := CategoryShareConfidence(scores, categoryIndex)

	if confidenceErr != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"classifier: unable to compute category confidence",
			confidenceErr,
		))
	}

	var strength float64
	strengthFound := false

	for wireIndex, wireInput := range datura.Peek[[]string](state, "inputs") {
		if wireInput != "strength" {
			continue
		}

		if scoreRoot == "features" {
			features := datura.Peek[[]float64](state, scoreRoot)

			if wireIndex >= len(features) {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"classifier: strength feature index out of range",
					nil,
				))
			}

			strength = features[wireIndex]
		}

		if scoreRoot != "features" {
			strength = datura.Peek[float64](state, scoreRoot, wireInput)
		}

		strengthFound = true
	}

	if !strengthFound {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"classifier: strength not in inputs",
			nil,
		))
	}

	if strength <= 0 || math.IsNaN(strength) || math.IsInf(strength, 0) {
		if !allZeroEvidence {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"classifier: strength must be positive and finite",
				nil,
			))
		}
	}

	state.MergeOutput("probabilities", probabilities)
	state.MergeOutput("category", categoryIndex)
	state.MergeOutput("confidence", confidence)
	state.MergeOutput("strength", strength)
	state.MergeOutput("value", categoryIndex)
	state.Poke("output", "root")

	outputInputs := make([]string, 0, len(inputs)+5)
	outputInputs = append(outputInputs, inputs...)
	outputInputs = append(outputInputs, "probabilities", "category", "confidence", "strength", "value")
	state.Poke(outputInputs, "inputs")

	return state.Read(payload)
}

func (classifier *Classifier) Write(p []byte) (int, error) {
	classifier.config.WithPayload(p)
	return len(p), nil
}

func (classifier *Classifier) Close() error {
	return nil
}

var _ io.ReadWriteCloser = (*Classifier)(nil)

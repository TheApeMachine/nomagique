package probability

import (
	"fmt"
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
	artifact     *datura.Artifact
	pendingFrame bool
	output       []byte
}

/*
NewClassifier returns a classifier wired from schema.inputs score keys.
*/
func NewClassifier(artifact *datura.Artifact) *Classifier {
	return &Classifier{artifact: artifact}
}

func (classifier *Classifier) Read(payload []byte) (int, error) {
	if len(classifier.output) > 0 {
		return classifier.readOutput(payload)
	}

	if !classifier.pendingFrame {
		return 0, io.EOF
	}

	state := datura.Acquire("classifier-state", datura.APPJSON)
	frame := classifier.artifact.DecryptPayload()

	if len(frame) == 0 {
		state.Release()
		classifier.pendingFrame = false

		return 0, io.EOF
	}

	if _, err := state.Unpack(frame); err != nil {
		state.Release()
		classifier.pendingFrame = false

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"classifier: state write failed",
			err,
		).With(
			append(
				classifier.artifact.Log(),
				"payload_bytes", len(frame),
				"payload_kind", classifier.payloadKind(frame),
			)...,
		))
	}

	defer state.Release()

	if err := classifier.Apply(state); err != nil {
		classifier.pendingFrame = false
		return 0, err
	}

	classifier.output = state.Pack()

	return classifier.readOutput(payload)
}

func (classifier *Classifier) readOutput(payload []byte) (int, error) {
	n := copy(payload, classifier.output)

	if n < len(classifier.output) {
		return n, io.ErrShortBuffer
	}

	classifier.output = nil
	classifier.pendingFrame = false

	return n, io.EOF
}

func (classifier *Classifier) payloadKind(frame []byte) string {
	if len(frame) == 0 {
		return "empty"
	}

	switch frame[0] {
	case '{', '[':
		return "json"
	default:
		return "packed"
	}
}

/*
Apply classifies scores already present on state and writes the classifier
outputs back onto the same artifact.
*/
func (classifier *Classifier) Apply(state *datura.Artifact) error {
	if classifier == nil || classifier.artifact == nil {
		return errnie.Error(errnie.Err(
			errnie.Validation,
			"classifier: config required",
			nil,
		))
	}

	if state == nil {
		return errnie.Error(errnie.Err(
			errnie.Validation,
			"classifier: state required",
			nil,
		))
	}

	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return errnie.Error(errnie.Err(
			errnie.Validation,
			"classifier: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return errnie.Error(errnie.Err(
			errnie.Validation,
			"classifier: inputs required",
			nil,
		))
	}

	categories := datura.Peek[[]string](classifier.artifact, "inputs")

	if len(categories) == 0 {
		return errnie.Error(errnie.Err(
			errnie.Validation,
			"classifier: inputs required",
			nil,
		))
	}

	categoryIndexes := datura.Peek[[]float64](classifier.artifact, "categoryIndexes")

	if len(categoryIndexes) > 0 && len(categoryIndexes) != len(categories) {
		return errnie.Error(errnie.Err(
			errnie.Validation,
			"classifier: categoryIndexes length must match inputs",
			nil,
		))
	}

	scores := make([]float64, len(categories))

	for index, category := range categories {
		if category == "" {
			return errnie.Error(errnie.Err(
				errnie.Validation,
				"classifier: empty input key",
				nil,
			))
		}

		var score float64
		scoreFound := false

		for wireIndex, wireInput := range inputs {
			if wireInput != category {
				continue
			}

			if rootKey == "features" {
				features := datura.Peek[[]float64](state, rootKey)

				if wireIndex >= len(features) {
					return errnie.Error(errnie.Err(
						errnie.Validation,
						"classifier: feature index out of range",
						nil,
					))
				}

				score = features[wireIndex]
			}

			if rootKey != "features" {
				score = datura.Peek[float64](state, rootKey, wireInput)
			}

			scoreFound = true
		}

		if !scoreFound {
			return errnie.Error(errnie.Err(
				errnie.Validation,
				"classifier: input not in inputs",
				nil,
			))
		}

		if math.IsNaN(score) || math.IsInf(score, 0) {
			return errnie.Error(errnie.Err(
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
		return errnie.Error(errnie.Err(
			errnie.Validation,
			"classifier: unable to compute softmax probabilities",
			err,
		))
	}

	winnerIndex := ArgmaxIndex(probabilities)
	categoryIndex := float64(winnerIndex + 1)

	if len(categoryIndexes) > 0 {
		categoryIndex = categoryIndexes[winnerIndex]
	}

	confidence, err := CategoryShareConfidence(scores, winnerIndex+1)

	if err != nil {
		return errnie.Error(errnie.Err(
			errnie.Validation,
			"classifier: unable to compute category confidence",
			err,
		))
	}

	var strength float64
	strengthFound := false

	for wireIndex, wireInput := range inputs {
		if wireInput != "strength" {
			continue
		}

		if rootKey == "features" {
			features := datura.Peek[[]float64](state, rootKey)

			if wireIndex >= len(features) {
				return errnie.Error(errnie.Err(
					errnie.Validation,
					"classifier: strength feature index out of range",
					nil,
				))
			}

			strength = features[wireIndex]
		}

		if rootKey != "features" {
			strength = datura.Peek[float64](state, rootKey, wireInput)
		}

		strengthFound = true
	}

	if !strengthFound {
		return errnie.Error(errnie.Err(
			errnie.Validation,
			"classifier: strength not in inputs",
			nil,
		))
	}

	if strength <= 0 || math.IsNaN(strength) || math.IsInf(strength, 0) {
		if !allZeroEvidence {
			return errnie.Error(errnie.Err(
				errnie.Validation,
				"classifier: strength must be positive and finite",
				nil,
			))
		}
	}

	outputs := map[string]any{
		"probabilities": probabilities,
		"category":      categoryIndex,
		"confidence":    confidence,
		"strength":      strength,
		"value":         categoryIndex,
	}

	for index, probability := range probabilities {
		wireCategoryIndex := float64(index + 1)

		if len(categoryIndexes) > 0 {
			wireCategoryIndex = categoryIndexes[index]
		}

		outputs[fmt.Sprintf("category.%d", int(wireCategoryIndex))] = probability
	}

	state.MergeOutputs(outputs)
	state.Poke("output", "root")

	outputInputs := make([]string, 0, len(categories)+len(probabilities)+5)
	outputInputs = append(outputInputs, categories...)
	outputInputs = append(outputInputs, "probabilities", "category", "confidence", "strength", "value")

	for index := range probabilities {
		wireCategoryIndex := float64(index + 1)

		if len(categoryIndexes) > 0 {
			wireCategoryIndex = categoryIndexes[index]
		}

		outputInputs = append(outputInputs, fmt.Sprintf("category.%d", int(wireCategoryIndex)))
	}

	state.Poke(outputInputs, "inputs")

	return nil
}

func (classifier *Classifier) Write(p []byte) (int, error) {
	if len(p) == 0 {
		classifier.pendingFrame = false
		classifier.output = nil

		return 0, nil
	}

	classifier.artifact.WithPayload(p)
	classifier.pendingFrame = true
	classifier.output = nil

	return len(p), nil
}

func (classifier *Classifier) Close() error {
	return nil
}

var _ io.ReadWriteCloser = (*Classifier)(nil)

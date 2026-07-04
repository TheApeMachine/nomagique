package probability

import (
	"fmt"
	"math"

	"github.com/theapemachine/errnie"
)

/*
ScoreClassifier classifies named float scores without knowing where they came
from. It is the data-shape agnostic core used by artifact and typed callers.
*/
type ScoreClassifier struct {
	inputs          []string
	categoryIndexes []float64
}

/*
ScoreResult is the full numeric classifier output.
*/
type ScoreResult struct {
	Value              float64
	Category           float64
	Confidence         float64
	ConfidenceBaseline float64
	EntryBaseline      float64
	ExitBaseline       float64
	Strength           float64
	Probabilities      []float64
	Distribution       map[string]float64
}

/*
NewScoreClassifier constructs a float-only classifier.
*/
func NewScoreClassifier(inputs []string, categoryIndexes []float64) *ScoreClassifier {
	return &ScoreClassifier{
		inputs:          append([]string(nil), inputs...),
		categoryIndexes: append([]float64(nil), categoryIndexes...),
	}
}

/*
Classify computes probabilities, category, confidence, and baselines from the
provided score map.
*/
func (classifier *ScoreClassifier) Classify(
	scores map[string]float64,
) (ScoreResult, error) {
	if len(classifier.inputs) == 0 {
		return ScoreResult{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"classifier: inputs required",
			nil,
		))
	}

	if len(classifier.categoryIndexes) > 0 &&
		len(classifier.categoryIndexes) != len(classifier.inputs) {
		return ScoreResult{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"classifier: categoryIndexes length must match inputs",
			nil,
		))
	}

	values := make([]float64, len(classifier.inputs))
	allZeroEvidence := true

	for index, input := range classifier.inputs {
		if input == "" {
			return ScoreResult{}, errnie.Error(errnie.Err(
				errnie.Validation,
				"classifier: empty input key",
				nil,
			))
		}

		score, ok := scores[input]
		if !ok {
			return ScoreResult{}, errnie.Error(errnie.Err(
				errnie.Validation,
				"classifier: missing input score: "+input,
				nil,
			))
		}

		if math.IsNaN(score) || math.IsInf(score, 0) {
			return ScoreResult{}, errnie.Error(errnie.Err(
				errnie.Validation,
				"classifier: score is non-finite",
				nil,
			))
		}

		if score > 0 {
			allZeroEvidence = false
		}

		values[index] = score
	}

	probabilities, err := SoftmaxScoresNormalized(values)
	if err != nil {
		return ScoreResult{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"classifier: unable to compute softmax probabilities",
			err,
		))
	}

	winnerIndex := ArgmaxIndex(probabilities)
	categoryIndex := float64(winnerIndex + 1)
	if len(classifier.categoryIndexes) > 0 {
		categoryIndex = classifier.categoryIndexes[winnerIndex]
	}

	confidence, err := CategoryShareConfidence(values, winnerIndex+1)
	if err != nil {
		return ScoreResult{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"classifier: unable to compute category confidence",
			err,
		))
	}

	strength := scores["strength"]
	if strength <= 0 || math.IsNaN(strength) || math.IsInf(strength, 0) {
		if !allZeroEvidence {
			return ScoreResult{}, errnie.Error(errnie.Err(
				errnie.Validation,
				"classifier: strength must be positive and finite",
				nil,
			))
		}
	}

	distribution := make(map[string]float64, len(probabilities))
	for index, probability := range probabilities {
		wireIndex := float64(index + 1)
		if len(classifier.categoryIndexes) > 0 {
			wireIndex = classifier.categoryIndexes[index]
		}

		distribution[fmt.Sprintf("%d", int(wireIndex))] = probability
	}

	baseline := 1.0 / float64(len(classifier.inputs))

	return ScoreResult{
		Value:              categoryIndex,
		Category:           categoryIndex,
		Confidence:         confidence,
		ConfidenceBaseline: baseline,
		EntryBaseline:      baseline,
		ExitBaseline:       baseline,
		Strength:           strength,
		Probabilities:      probabilities,
		Distribution:       distribution,
	}, nil
}

func (result ScoreResult) Outputs() map[string]any {
	return map[string]any{
		"probabilities":       result.Probabilities,
		"category":            result.Category,
		"confidence":          result.Confidence,
		"confidence_baseline": result.ConfidenceBaseline,
		"distribution":        result.Distribution,
		"entry_baseline":      result.EntryBaseline,
		"exit_baseline":       result.ExitBaseline,
		"strength":            result.Strength,
		"value":               result.Value,
	}
}

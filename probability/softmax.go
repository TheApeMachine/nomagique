package probability

import (
	"fmt"
	"math"

	"github.com/theapemachine/errnie"
)

/*
SoftmaxConfig declares the category order for probability output.
*/
type SoftmaxConfig struct {
	Inputs    []string
	Normalize bool
}

/*
SoftmaxInput carries competing category scores.
*/
type SoftmaxInput struct {
	Scores []CategoryScore
}

/*
SoftmaxOutput carries normalized category probabilities.
*/
type SoftmaxOutput struct {
	Probabilities []CategoryScore
	Values        []float64
}

/*
Softmax maps competing scores to a normalized probability vector.
*/
type Softmax struct {
	config SoftmaxConfig
}

/*
NewSoftmax returns a typed softmax calculator.
*/
func NewSoftmax(config SoftmaxConfig) *Softmax {
	return &Softmax{
		config: config,
	}
}

/*
Measure returns configured category probabilities for the input scores.
*/
func (softmax *Softmax) Measure(input SoftmaxInput) (SoftmaxOutput, error) {
	scores, err := softmax.scores(input)

	if err != nil {
		return SoftmaxOutput{}, err
	}

	probabilities, err := softmax.distribute(scores, softmax.config.Normalize)

	if err != nil {
		return SoftmaxOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"softmax: unable to compute probabilities",
			err,
		))
	}

	return softmax.output(probabilities), nil
}

/*
scores aligns input categories to the configured key order so Measure always
feeds distribute a fixed-length vector even when callers omit categories.
*/
func (softmax *Softmax) scores(input SoftmaxInput) ([]float64, error) {
	if softmax == nil || len(softmax.config.Inputs) == 0 {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"softmax: inputs required",
			nil,
		))
	}

	scores := make([]float64, len(softmax.config.Inputs))

	for index, category := range softmax.config.Inputs {
		if category == "" {
			return nil, errnie.Error(errnie.Err(
				errnie.Validation,
				"softmax: empty input key",
				nil,
			))
		}

		score, err := input.Score(category)
		if err != nil {
			return nil, err
		}

		scores[index] = score
	}

	return scores, nil
}

/*
Score resolves one category's logit and rejects duplicate keys so softmax
normalization cannot double-count the same semantic outcome.
*/
func (input SoftmaxInput) Score(category string) (float64, error) {
	found := false
	score := 0.0

	for _, candidate := range input.Scores {
		if candidate.Category != category {
			continue
		}

		if found {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"softmax: duplicate input key",
				nil,
			))
		}

		score = candidate.Score
		found = true
	}

	if !found {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"softmax: input score missing",
			nil,
		))
	}

	if err := finiteProbability("softmax", score); err != nil {
		return 0, err
	}

	return score, nil
}

/*
output reattaches category labels to the probability vector for stable exports.
*/
func (softmax *Softmax) output(probabilities []float64) SoftmaxOutput {
	output := SoftmaxOutput{
		Probabilities: make([]CategoryScore, len(softmax.config.Inputs)),
		Values:        append([]float64(nil), probabilities...),
	}

	for index, category := range softmax.config.Inputs {
		output.Probabilities[index] = CategoryScore{
			Category: category,
			Score:    probabilities[index],
		}
	}

	return output
}

/*
distribute validates finiteness then chooses raw in-process softmax versus
the shared normalized path when config demands unit-sum outputs.
*/
func (softmax *Softmax) distribute(scores []float64, normalize bool) ([]float64, error) {
	if len(scores) == 0 {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"softmax: requires at least one score",
			nil,
		))
	}

	for index, score := range scores {
		if math.IsNaN(score) || math.IsInf(score, 0) {
			return nil, errnie.Error(errnie.Err(
				errnie.Validation,
				fmt.Sprintf("softmax: score[%d] is non-finite", index),
				nil,
			))
		}
	}

	if normalize {
		return softmax.normalized(scores)
	}

	return softmax.raw(scores)
}

/*
raw applies log-sum-exp stabilization before normalizing so large logits do
not overflow while still producing a proper probability mass.
*/
func (softmax *Softmax) raw(scores []float64) ([]float64, error) {
	probabilities := make([]float64, len(scores))
	maxScore := scores[0]

	for _, score := range scores[1:] {
		maxScore = math.Max(maxScore, score)
	}

	expSum := 0.0

	for index, score := range scores {
		weighted := math.Exp(score - maxScore)
		probabilities[index] = weighted
		expSum += weighted
	}

	if expSum <= 0 {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"softmax: normalization sum is non-positive",
			nil,
		))
	}

	for index := range probabilities {
		probabilities[index] /= expSum
	}

	return probabilities, nil
}

/*
normalized delegates to SoftmaxScoresNormalized so batch and streaming callers
share one numerically vetted implementation.
*/
func (softmax *Softmax) normalized(scores []float64) ([]float64, error) {
	return SoftmaxScoresNormalized(scores)
}

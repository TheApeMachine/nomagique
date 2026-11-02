package probability

import (
	"math"

	"github.com/theapemachine/errnie"
)

func SoftmaxScoresNormalized(scores []float64) ([]float64, error) {
	if len(scores) == 0 {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"softmax: requires at least one score",
			nil,
		))
	}

	maxScore := scores[0]

	for index, score := range scores {
		if math.IsNaN(score) || math.IsInf(score, 0) {
			return nil, errnie.Error(errnie.Err(
				errnie.Validation,
				"softmax: score is non-finite",
				nil,
			).With("index", index))
		}

		maxScore = math.Max(maxScore, score)
	}

	if len(scores) == 1 {
		return []float64{1}, nil
	}

	probabilities := make([]float64, len(scores))
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

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

	for index, score := range scores {
		if math.IsNaN(score) || math.IsInf(score, 0) {
			return nil, errnie.Error(errnie.Err(
				errnie.Validation,
				"softmax: score is non-finite",
				nil,
			).With("index", index))
		}
	}

	if len(scores) == 1 {
		return []float64{1}, nil
	}

	mean := 0.0

	for _, score := range scores {
		mean += score
	}

	mean /= float64(len(scores))

	variance := 0.0

	for _, score := range scores {
		deviation := score - mean
		variance += deviation * deviation
	}

	variance /= float64(len(scores) - 1)
	stddev := math.Sqrt(variance)

	if stddev <= 0 {
		share := 1.0 / float64(len(scores))
		probabilities := make([]float64, len(scores))

		for index := range probabilities {
			probabilities[index] = share
		}

		return probabilities, nil
	}

	standardized := make([]float64, len(scores))

	for index, score := range scores {
		standardized[index] = (score - mean) / stddev
	}

	maxScore := standardized[0]

	for _, score := range standardized[1:] {
		maxScore = math.Max(maxScore, score)
	}

	probabilities := make([]float64, len(scores))
	expSum := 0.0

	for index, score := range standardized {
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

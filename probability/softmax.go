package probability

import (
	"fmt"
	"math"
)

/*
SoftmaxScores maps raw scores to a normalized probability vector.
*/
func SoftmaxScores(scores []float64) ([]float64, error) {
	if len(scores) == 0 {
		return nil, fmt.Errorf("probability: softmax requires at least one score")
	}

	for index, score := range scores {
		if math.IsNaN(score) || math.IsInf(score, 0) {
			return nil, fmt.Errorf("probability: softmax score[%d] is non-finite", index)
		}
	}

	probabilities := make([]float64, len(scores))
	maxScore := scores[0]

	for _, score := range scores[1:] {
		if score > maxScore {
			maxScore = score
		}
	}

	expSum := 0.0

	for index, score := range scores {
		weighted := math.Exp(score - maxScore)
		probabilities[index] = weighted
		expSum += weighted
	}

	if expSum <= 0 {
		return nil, fmt.Errorf("probability: softmax normalization sum is non-positive")
	}

	for index := range probabilities {
		probabilities[index] /= expSum
	}

	return probabilities, nil
}

func uniformProbabilities(count int) []float64 {
	probabilities := make([]float64, count)
	share := 1.0 / float64(count)

	for index := range probabilities {
		probabilities[index] = share
	}

	return probabilities
}

/*
SoftmaxScoresNormalized standardizes scores by their own spread before applying
softmax, so the resulting probabilities reflect how decisively the winning score
SEPARATES from the field rather than the raw magnitude of the scores.

Plain SoftmaxScores saturates to a one-hot vector whenever one raw score is much
larger than the others — which happens routinely when an input is an unbounded
physical quantity (relative volume, divergence, a 1/spread ratio): the winner's
confidence pins to ~1.0 and surprise collapses to 0, regardless of how genuinely
distinct the state is. Dividing the centered scores by their sample standard
deviation makes the margin scale-invariant: a 2x and a 50x volume spike that both
clearly mean "ignition" yield comparable, sub-unity confidence instead of both
pinning to exactly 1.0.

When every score is equal (zero spread) the result is the uniform distribution.
*/
func SoftmaxScoresNormalized(scores []float64) ([]float64, error) {
	if len(scores) == 0 {
		return nil, fmt.Errorf("probability: softmax requires at least one score")
	}

	for index, score := range scores {
		if math.IsNaN(score) || math.IsInf(score, 0) {
			return nil, fmt.Errorf("probability: softmax score[%d] is non-finite", index)
		}
	}

	if len(scores) == 1 {
		return uniformProbabilities(1), nil
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
		return uniformProbabilities(len(scores)), nil
	}

	standardized := make([]float64, len(scores))

	for index, score := range scores {
		standardized[index] = (score - mean) / stddev
	}

	return SoftmaxScores(standardized)
}

/*
ArgmaxIndex returns the index of the largest value.
*/
func ArgmaxIndex(values []float64) int {
	if len(values) == 0 {
		return 0
	}

	bestIndex := 0
	bestValue := values[0]

	for index, value := range values[1:] {
		if value > bestValue {
			bestValue = value
			bestIndex = index + 1
		}
	}

	return bestIndex
}

/*
CategoryConfidence returns the softmax probability for the selected category.
categoryIndex is 1-based; when zero, the winning category probability is used.
*/
func CategoryConfidence(probabilities []float64, categoryIndex int) (float64, error) {
	if len(probabilities) == 0 {
		return 0, fmt.Errorf("probability: category confidence requires probabilities")
	}

	probabilityIndex := ArgmaxIndex(probabilities)

	if categoryIndex > 0 && categoryIndex-1 < len(probabilities) {
		probabilityIndex = categoryIndex - 1
	}

	return probabilities[probabilityIndex], nil
}

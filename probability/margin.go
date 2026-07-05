package probability

import "fmt"

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
CompetitionMargin scores how decisively excess separates from a reference span.
*/
func CompetitionMargin(excess, span float64) (float64, error) {
	if excess <= 0 || span <= 0 {
		return 0, fmt.Errorf("probability: competition margin requires positive excess and span")
	}

	return excess / (excess + span), nil
}

/*
MagnitudeMargin maps a positive magnitude into (0, 1).
*/
func MagnitudeMargin(value float64) (float64, error) {
	if value <= 0 {
		return 0, fmt.Errorf("probability: magnitude margin requires positive value")
	}

	return value / (1 + value), nil
}

/*
CategoryShareConfidence returns the selected category's share of total category
evidence. Each category carries one unit of pseudocount, so N equal scores yield
1/N, close calls stay near 1/N, and a lone category with finite evidence cannot
reach 1.0. categoryIndex is 1-based; when zero, the winning category is used.
*/
func CategoryShareConfidence(scores []float64, categoryIndex int) (float64, error) {
	if len(scores) == 0 {
		return 0, fmt.Errorf("probability: category share confidence requires scores")
	}

	index := ArgmaxIndex(scores)

	if categoryIndex > 0 && categoryIndex-1 < len(scores) {
		index = categoryIndex - 1
	}

	if index < 0 || index >= len(scores) {
		return 0, fmt.Errorf("probability: category share confidence index out of range")
	}

	selected := scores[index]

	if selected <= 0 {
		evidenceSum := 0.0

		for _, score := range scores {
			if score > 0 {
				evidenceSum += score
			}
		}

		if evidenceSum <= 0 {
			return 1.0 / float64(len(scores)), nil
		}

		return 0, fmt.Errorf("probability: category share confidence requires positive selected evidence")
	}

	evidenceSum := 0.0

	for _, score := range scores {
		if score > 0 {
			evidenceSum += score
		}
	}

	return (selected + 1) / (evidenceSum + float64(len(scores))), nil
}

/*
CategoryEvidenceBaselines derives confidence gates from the current evidence
competition instead of from category count. The entry gate is the nearest
competitor's posterior share; the exit gate is the average non-winning evidence
floor, so protective decisions can react before the strongest competitor wins.
categoryIndex is 1-based; when zero, the winning category is used.
*/
func CategoryEvidenceBaselines(
	scores []float64,
	categoryIndex int,
) (float64, float64, float64, error) {
	if len(scores) == 0 {
		return 0, 0, 0, fmt.Errorf("probability: category baselines require scores")
	}

	index := ArgmaxIndex(scores)

	if categoryIndex > 0 && categoryIndex-1 < len(scores) {
		index = categoryIndex - 1
	}

	if index < 0 || index >= len(scores) {
		return 0, 0, 0, fmt.Errorf("probability: category baseline index out of range")
	}

	evidenceSum := 0.0
	runnerUp := 0.0
	nonWinningSum := 0.0
	nonWinningCount := 0

	for scoreIndex, score := range scores {
		if score <= 0 {
			if scoreIndex != index {
				nonWinningCount++
			}

			continue
		}

		evidenceSum += score

		if scoreIndex == index {
			continue
		}

		nonWinningCount++
		nonWinningSum += score

		if score > runnerUp {
			runnerUp = score
		}
	}

	denominator := evidenceSum + float64(len(scores))

	if denominator <= 0 {
		return 0, 0, 0, fmt.Errorf("probability: category baseline denominator required")
	}

	entryBaseline := (runnerUp + 1) / denominator
	exitEvidence := runnerUp

	if nonWinningCount > 0 {
		exitEvidence = nonWinningSum / float64(nonWinningCount)
	}

	exitBaseline := (exitEvidence + 1) / denominator
	confidenceBaseline := exitBaseline

	return confidenceBaseline, entryBaseline, exitBaseline, nil
}

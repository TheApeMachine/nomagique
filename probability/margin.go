package probability

import "fmt"

/*
CompetitionMargin scores how decisively excess separates from a reference span.
*/
func CompetitionMargin(excess, span float64) float64 {
	if excess <= 0 || span <= 0 {
		return 0
	}

	return excess / (excess + span)
}

/*
MagnitudeMargin maps a positive magnitude into (0, 1).
*/
func MagnitudeMargin(value float64) float64 {
	if value <= 0 {
		return 0
	}

	return value / (1 + value)
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
		return 0, nil
	}

	evidenceSum := 0.0

	for _, score := range scores {
		if score > 0 {
			evidenceSum += score
		}
	}

	return (selected + 1) / (evidenceSum + float64(len(scores))), nil
}

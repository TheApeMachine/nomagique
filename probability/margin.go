package probability

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

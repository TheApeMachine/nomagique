package correlation

import "math"

/*
intervalCorrelationSlices combines realized variances with overlap covariance
so interval-based series correlate when returns live on distinct time spans.
*/
func intervalCorrelationSlices(
	leftStarts, leftEnds, leftRets,
	rightStarts, rightEnds, rightRets []float64,
) (float64, bool) {
	varLeft := realizedVariance(leftRets)
	varRight := realizedVariance(rightRets)

	if varLeft <= 0 || varRight <= 0 {
		return 0, false
	}

	covariance := intervalCovariance(leftStarts, leftEnds, leftRets, rightStarts, rightEnds, rightRets)
	correlation := covariance / math.Sqrt(varLeft*varRight)

	if correlation > 1 {
		return 1, true
	}

	if correlation < -1 {
		return -1, true
	}

	return correlation, true
}

/*
realizedVariance sums squared returns as the Hayashi-style variance term when
means are implicit in log-return products.
*/
func realizedVariance(rets []float64) float64 {
	total := 0.0

	for _, ret := range rets {
		total += ret * ret
	}

	return total
}

/*
intervalCovariance walks overlapping time windows between two interval series,
advancing a sliding right index so complexity stays linear in event count.
*/
func intervalCovariance(
	leftStarts, leftEnds, leftRets,
	rightStarts, rightEnds, rightRets []float64,
) float64 {
	covariance := 0.0
	window := 0

	for leftIndex := range leftStarts {
		leftStart := int64(leftStarts[leftIndex])
		leftEnd := int64(leftEnds[leftIndex])
		leftRet := leftRets[leftIndex]

		for window < len(rightStarts) && int64(rightEnds[window]) <= leftStart {
			window++
		}

		for index := window; index < len(rightStarts) && int64(rightStarts[index]) < leftEnd; index++ {
			covariance += leftRet * rightRets[index]
		}
	}

	return covariance
}

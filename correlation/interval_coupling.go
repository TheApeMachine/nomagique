package correlation

import "math"

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

func realizedVariance(rets []float64) float64 {
	total := 0.0

	for _, ret := range rets {
		total += ret * ret
	}

	return total
}

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

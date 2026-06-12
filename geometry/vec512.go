package geometry

import "math"

func vecMul(dst, left, right []float64) {
	for index := range dst {
		dst[index] = left[index] * right[index]
	}
}

func vecAdd(dst, left, right []float64) {
	for index := range dst {
		dst[index] = left[index] + right[index]
	}
}

func vecSqrt(dst, src []float64) {
	for index := range dst {
		dst[index] = math.Sqrt(src[index])
	}
}

func vecAtan2(dst, left, right []float64) {
	for index := range dst {
		dst[index] = math.Atan2(left[index], right[index])
	}
}

func vecScale(dst, src []float64, scalar float64) {
	for index := range dst {
		dst[index] = src[index] * scalar
	}
}

func vecAddScalar(dst, src []float64, scalar float64) {
	for index := range dst {
		dst[index] = src[index] + scalar
	}
}

func vecMax(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	maxValue := values[0]

	for index := 1; index < len(values); index++ {
		if values[index] > maxValue {
			maxValue = values[index]
		}
	}

	return maxValue
}

func vecSum(values []float64) float64 {
	var total float64

	for _, value := range values {
		total += value
	}

	return total
}

func vecDotProduct(left, right []float64) float64 {
	var total float64

	for index := range left {
		total += left[index] * right[index]
	}

	return total
}

func vecSumOfSquares(values []float64) float64 {
	var total float64

	for _, value := range values {
		total += value * value
	}

	return total
}

func vecCosInPlace(values []float64) {
	for index := range values {
		values[index] = math.Cos(values[index])
	}
}

func vecSinCos(sinDst, cosDst, phases []float64) {
	for index := range phases {
		sinDst[index], cosDst[index] = math.Sincos(phases[index])
	}
}

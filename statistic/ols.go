package statistic

import (
	"math"

	"gonum.org/v1/gonum/stat"
)

const minBackdoorDenominator = 1e-9

/*
OLS fits target against predictors via ridge-regularized normal equations.
*/
func OLS(target []float64, predictors ...[]float64) ([]float64, bool) {
	if len(target) < 2 {
		return nil, false
	}

	for _, predictor := range predictors {
		if len(predictor) != len(target) {
			return nil, false
		}
	}

	size := len(target)
	width := len(predictors) + 1
	normal := make([][]float64, width)

	for row := range width {
		normal[row] = make([]float64, width)
	}

	targetVec := make([]float64, width)
	rowValues := make([]float64, width)

	for index := 0; index < size; index++ {
		rowValues[0] = 1

		for predictorIndex, predictor := range predictors {
			rowValues[predictorIndex+1] = predictor[index]
		}

		for row := 0; row < width; row++ {
			targetVec[row] += rowValues[row] * target[index]

			for col := 0; col < width; col++ {
				normal[row][col] += rowValues[row] * rowValues[col]
			}
		}
	}

	solution, err := NewRidgeSolver().Solve(normal, targetVec)

	if err != nil {
		return nil, false
	}

	return solution, true
}

/*
Residualize removes linear dependence on controls from target.
*/
func Residualize(target []float64, controls ...[]float64) ([]float64, bool) {
	if len(controls) == 0 {
		return append([]float64(nil), target...), true
	}

	coefficients, ok := OLS(target, controls...)

	if !ok {
		return nil, false
	}

	residuals := make([]float64, len(target))

	for index := range target {
		fitted := coefficients[0]

		for controlIndex, control := range controls {
			fitted += coefficients[controlIndex+1] * control[index]
		}

		residuals[index] = target[index] - fitted
	}

	return residuals, true
}

/*
Dot returns the inner product of two equal-length vectors.
*/
func Dot(left, right []float64) float64 {
	if len(left) != len(right) {
		return 0
	}

	sum := 0.0

	for index := range left {
		sum += left[index] * right[index]
	}

	return sum
}

/*
PairConditionNumber returns (1 + |r|) / (1 - |r|) for two columns.
*/
func PairConditionNumber(left, right []float64) (float64, bool) {
	if len(left) != len(right) || len(left) == 0 {
		return 0, false
	}

	correlation := math.Abs(stat.Correlation(left, right, nil))

	if correlation >= 1 {
		return math.Inf(1), true
	}

	return (1 + correlation) / (1 - correlation), true
}

/*
BackdoorDenominator floors small residual norms during backdoor estimation.
*/
func BackdoorDenominator(residualNorm float64) float64 {
	return math.Max(residualNorm, minBackdoorDenominator)
}

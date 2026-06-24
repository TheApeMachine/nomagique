package causal

import (
	"math"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/statistic"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/stat"
)

func backdoorDenominator(residualNorm float64) (float64, error) {
	if math.IsNaN(residualNorm) || math.IsInf(residualNorm, 0) || residualNorm <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal: backdoor denominator is non-positive",
			nil,
		))
	}

	return residualNorm, nil
}

func olsFit(target []float64, predictors ...[]float64) ([]float64, error) {
	if len(target) < 2 {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal: ols requires at least two target samples",
			nil,
		))
	}

	for _, value := range target {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, errnie.Error(errnie.Err(
				errnie.Validation,
				"causal: ols target is non-finite",
				nil,
			))
		}
	}

	for _, predictor := range predictors {
		if len(predictor) != len(target) {
			return nil, errnie.Error(errnie.Err(
				errnie.Validation,
				"causal: ols predictor length mismatch",
				nil,
			))
		}

		for _, value := range predictor {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return nil, errnie.Error(errnie.Err(
					errnie.Validation,
					"causal: ols predictor is non-finite",
					nil,
				))
			}
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

	solution, err := statistic.NewRidgeSolver().Solve(normal, targetVec)

	if err != nil {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal: ols ridge solve failed",
			err,
		))
	}

	return solution, nil
}

func residualize(target []float64, controls ...[]float64) ([]float64, error) {
	if len(controls) == 0 {
		return append([]float64(nil), target...), nil
	}

	coefficients, err := olsFit(target, controls...)

	if err != nil {
		return nil, err
	}

	residuals := make([]float64, len(target))

	for index := range target {
		fitted := coefficients[0]

		for controlIndex, control := range controls {
			fitted += coefficients[controlIndex+1] * control[index]
		}

		residuals[index] = target[index] - fitted
	}

	return residuals, nil
}

func vectorDot(left, right []float64) (float64, error) {
	if len(left) != len(right) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal: dot length mismatch",
			nil,
		))
	}

	return floats.Dot(left, right), nil
}

func pairConditionNumber(left, right []float64) (float64, error) {
	if len(left) != len(right) || len(left) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal: pair condition number requires equal non-empty columns",
			nil,
		))
	}

	correlation := math.Abs(stat.Correlation(left, right, nil))

	if math.IsNaN(correlation) || math.IsInf(correlation, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal: pair correlation is non-finite",
			nil,
		))
	}

	if correlation >= 1 {
		return math.Inf(1), nil
	}

	return (1 + correlation) / (1 - correlation), nil
}

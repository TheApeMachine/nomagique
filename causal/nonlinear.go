package causal

import (
	"errors"
	"sort"

	"gonum.org/v1/gonum/stat"
)

const nonLinearStumps = 8

type stumpSplit struct {
	featureIndex int
	threshold    float64
	leftMean     float64
	rightMean    float64
}

/*
NonLinearModel is a gradient-boosted stump ensemble for target prediction.
*/
type NonLinearModel struct {
	intercept float64
	stumps    []stumpSplit
}

/*
FitNonLinearTable fits a stump ensemble on the configured node table.
*/
func FitNonLinearTable(nodeTable NodeTable, features []int) (NonLinearModel, bool) {
	if len(features) == 0 {
		return NonLinearModel{}, false
	}

	if err := nodeTable.validateNodes(features...); err != nil {
		return NonLinearModel{}, false
	}

	targets, err := nodeTable.column(nodeTable.target)

	if err != nil {
		return NonLinearModel{}, false
	}

	residuals := append([]float64(nil), targets...)
	thresholds := featureThresholds(nodeTable, features)
	model := NonLinearModel{
		intercept: stat.Mean(targets, nil),
		stumps:    make([]stumpSplit, 0, nonLinearStumps),
	}

	for stumpIndex := 0; stumpIndex < nonLinearStumps; stumpIndex++ {
		split, gain := bestStump(nodeTable, residuals, features, thresholds)

		if gain <= 0 {
			break
		}

		model.stumps = append(model.stumps, split)

		for index, row := range nodeTable.rows {
			residuals[index] -= stumpPredictionRow(row, split, -1, 0)
		}
	}

	return model, len(model.stumps) > 0
}

/*
Predict returns the ensemble prediction with optional treatment override.
*/
func (model NonLinearModel) Predict(
	row []float64, overrideNode int, overrideValue float64,
) (float64, error) {
	prediction := model.intercept

	for _, split := range model.stumps {
		if split.featureIndex < 0 || split.featureIndex >= len(row) {
			return 0, errors.New("causal: stump feature outside row")
		}

		prediction += stumpPredictionRow(row, split, overrideNode, overrideValue)
	}

	return prediction, nil
}

/*
CounterfactualUplift returns predicted change under a treatment intervention.
*/
func (model NonLinearModel) CounterfactualUplift(
	row []float64, treatment int, intervention float64,
) (float64, error) {
	observed, err := model.Predict(row, -1, 0)

	if err != nil {
		return 0, err
	}

	counterfactual, err := model.Predict(row, treatment, intervention)

	if err != nil {
		return 0, err
	}

	return counterfactual - observed, nil
}

func bestStump(
	nodeTable NodeTable,
	residuals []float64,
	features []int,
	thresholds map[int][]float64,
) (stumpSplit, float64) {
	best := stumpSplit{}
	bestGain := 0.0

	for _, featureIndex := range features {
		for _, threshold := range thresholds[featureIndex] {
			leftSum, leftCount, rightSum, rightCount := partitionResiduals(
				nodeTable.rows, residuals, featureIndex, threshold,
			)

			if leftCount == 0 || rightCount == 0 {
				continue
			}

			leftMean := leftSum / leftCount
			rightMean := rightSum / rightCount
			gain := splitGain(
				residuals, leftMean, rightMean, nodeTable.rows, featureIndex, threshold,
			)

			if gain <= bestGain {
				continue
			}

			bestGain = gain
			best = stumpSplit{
				featureIndex: featureIndex,
				threshold:    threshold,
				leftMean:     leftMean,
				rightMean:    rightMean,
			}
		}
	}

	return best, bestGain
}

func featureThresholds(nodeTable NodeTable, features []int) map[int][]float64 {
	thresholds := make(map[int][]float64, len(features))

	for _, featureIndex := range features {
		seen := make(map[float64]struct{}, len(nodeTable.rows))

		for _, row := range nodeTable.rows {
			value := featureValue(row, featureIndex)
			seen[value] = struct{}{}
		}

		values := make([]float64, 0, len(seen))

		for value := range seen {
			values = append(values, value)
		}

		sort.Float64s(values)
		thresholds[featureIndex] = values
	}

	return thresholds
}

func partitionResiduals(
	rows [][]float64,
	residuals []float64,
	featureIndex int,
	threshold float64,
) (leftSum, leftCount, rightSum, rightCount float64) {
	for index, row := range rows {
		if featureValue(row, featureIndex) <= threshold {
			leftSum += residuals[index]
			leftCount++
			continue
		}

		rightSum += residuals[index]
		rightCount++
	}

	return leftSum, leftCount, rightSum, rightCount
}

func splitGain(
	residuals []float64,
	leftMean, rightMean float64,
	rows [][]float64,
	featureIndex int,
	threshold float64,
) float64 {
	before := 0.0
	after := 0.0

	for index, row := range rows {
		residual := residuals[index]
		before += residual * residual
		prediction := rightMean

		if featureValue(row, featureIndex) <= threshold {
			prediction = leftMean
		}

		delta := residual - prediction
		after += delta * delta
	}

	return before - after
}

func stumpPredictionRow(
	row []float64,
	split stumpSplit,
	overrideNode int,
	overrideValue float64,
) float64 {
	value := featureValueWithOverride(row, split.featureIndex, overrideNode, overrideValue)

	if value <= split.threshold {
		return split.leftMean
	}

	return split.rightMean
}

func featureValue(row []float64, featureIndex int) float64 {
	return featureValueWithOverride(row, featureIndex, -1, 0)
}

func featureValueWithOverride(
	row []float64,
	featureIndex int,
	overrideNode int,
	overrideValue float64,
) float64 {
	if featureIndex == overrideNode {
		return overrideValue
	}

	return row[featureIndex]
}

package causal

import (
	"fmt"
	"io"
	"math"
	"sort"

	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

type nodeTable struct {
	rows   [][]float64
	target int
}

type linearModel struct {
	coefficients []float64
	predictors   []int
}

func newNodeTable(rows [][]float64, target, minRows int) (nodeTable, error) {
	if minRows <= 0 {
		return nodeTable{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal: minRows must be positive",
			nil,
		))
	}

	if len(rows) < minRows {
		return nodeTable{}, errnie.Error(errnie.Err(
			errnie.Validation,
			fmt.Sprintf("causal: table needs %d rows, got %d", minRows, len(rows)),
			nil,
		))
	}

	nodeCount := len(rows[0])

	if nodeCount == 0 {
		return nodeTable{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal: rows must contain nodes",
			nil,
		))
	}

	if target < 0 || target >= nodeCount {
		return nodeTable{}, errnie.Error(errnie.Err(
			errnie.Validation,
			fmt.Sprintf("causal: target node %d outside row width %d", target, nodeCount),
			nil,
		))
	}

	for rowIndex, row := range rows {
		if len(row) != nodeCount {
			return nodeTable{}, errnie.Error(errnie.Err(
				errnie.Validation,
				fmt.Sprintf(
					"causal: row %d width %d differs from %d",
					rowIndex, len(row), nodeCount,
				),
				nil,
			))
		}
	}

	return nodeTable{rows: rows, target: target}, nil
}

func (nodeTable nodeTable) association(treatment int) (float64, error) {
	treatmentValues, err := nodeTable.column(treatment)

	if err != nil {
		return 0, err
	}

	targetValues, err := nodeTable.column(nodeTable.target)

	if err != nil {
		return 0, err
	}

	for _, value := range treatmentValues {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"causal: treatment sample is non-finite",
				nil,
			))
		}
	}

	for _, value := range targetValues {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"causal: target sample is non-finite",
				nil,
			))
		}
	}

	if stat.Variance(treatmentValues, nil) <= 0 || stat.Variance(targetValues, nil) <= 0 {
		return 0, io.EOF
	}

	association := stat.Correlation(treatmentValues, targetValues, nil)

	if math.IsNaN(association) || math.IsInf(association, 0) {
		return 0, io.EOF
	}

	return association, nil
}

func (nodeTable nodeTable) backdoorEffect(treatment int, controls ...int) (float64, error) {
	treatmentValues, err := nodeTable.column(treatment)

	if err != nil {
		return 0, err
	}

	targetValues, err := nodeTable.column(nodeTable.target)

	if err != nil {
		return 0, err
	}

	controlColumns, err := nodeTable.columns(controls...)

	if err != nil {
		return 0, err
	}

	residualTarget, err := residualize(targetValues, controlColumns...)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal: target residualization failed",
			err,
		))
	}

	residualTreatment, err := residualize(treatmentValues, controlColumns...)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal: treatment residualization failed",
			err,
		))
	}

	dotTreatment, err := vectorDot(residualTreatment, residualTreatment)

	if err != nil {
		return 0, err
	}

	denominator, err := backdoorDenominator(dotTreatment)

	if err != nil {
		return 0, err
	}

	dotTarget, err := vectorDot(residualTarget, residualTreatment)

	if err != nil {
		return 0, err
	}

	return dotTarget / denominator, nil
}

func (nodeTable nodeTable) fitLinearModel(predictors ...int) (linearModel, error) {
	targetValues, err := nodeTable.column(nodeTable.target)

	if err != nil {
		return linearModel{}, err
	}

	predictorColumns, err := nodeTable.columns(predictors...)

	if err != nil {
		return linearModel{}, err
	}

	coefficients, err := olsFit(targetValues, predictorColumns...)

	if err != nil {
		return linearModel{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal: linear structural fit failed",
			err,
		))
	}

	return linearModel{
		coefficients: coefficients,
		predictors:   append([]int(nil), predictors...),
	}, nil
}

func (nodeTable nodeTable) pairConditionNumber(left, right int) (float64, error) {
	leftColumn, err := nodeTable.column(left)

	if err != nil {
		return 0, err
	}

	rightColumn, err := nodeTable.column(right)

	if err != nil {
		return 0, err
	}

	condition, err := pairConditionNumber(leftColumn, rightColumn)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal: pair condition number failed",
			err,
		))
	}

	return condition, nil
}

func (nodeTable nodeTable) percentile(node int, percentile float64) (float64, error) {
	values, err := nodeTable.column(node)

	if err != nil {
		return 0, err
	}

	if len(values) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal: percentile node has no values",
			nil,
		))
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	for _, value := range sorted {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"causal: percentile sample is non-finite",
				nil,
			))
		}
	}

	return stat.Quantile(percentile, stat.LinInterp, sorted, nil), nil
}

func (nodeTable nodeTable) kernelBackdoorEffect(
	treatment int, bandwidth float64, controls ...int,
) (float64, error) {
	if bandwidth <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal: kernel bandwidth must be positive",
			nil,
		))
	}

	if err := nodeTable.validateNode(treatment); err != nil {
		return 0, err
	}

	if err := nodeTable.validateNodes(controls...); err != nil {
		return 0, err
	}

	targetValues, err := nodeTable.column(nodeTable.target)

	if err != nil {
		return 0, err
	}

	treatmentValues, err := nodeTable.column(treatment)

	if err != nil {
		return 0, err
	}

	controlColumns, err := nodeTable.columns(controls...)

	if err != nil {
		return 0, err
	}

	residualTarget, err := residualize(targetValues, controlColumns...)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal: target residualization failed",
			err,
		))
	}

	residualTreatment, err := residualize(treatmentValues, controlColumns...)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal: treatment residualization failed",
			err,
		))
	}

	current := nodeTable.rows[len(nodeTable.rows)-1]
	numerator := 0.0
	weightSum := 0.0

	for rowIndex, row := range nodeTable.rows {
		distanceSum := 0.0

		for _, feature := range controls {
			delta := current[feature] - row[feature]
			distanceSum += delta * delta
		}

		distance := math.Sqrt(distanceSum)
		weight := math.Exp(-distance * distance / (2 * bandwidth * bandwidth))

		if math.IsNaN(weight) || math.IsInf(weight, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"causal: kernel weight is non-finite",
				nil,
			))
		}

		if weight <= 0 {
			continue
		}

		treatmentValue := residualTreatment[rowIndex]
		numerator += weight * residualTarget[rowIndex] * treatmentValue
		weightSum += weight * treatmentValue * treatmentValue
	}

	if weightSum <= 0 {
		return 0, io.EOF
	}

	denominator, err := backdoorDenominator(weightSum)

	if err != nil {
		return 0, err
	}

	return numerator / denominator, nil
}

func (model linearModel) predict(
	row []float64, overrideNode int, overrideValue float64,
) (float64, error) {
	if len(model.coefficients) != len(model.predictors)+1 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal: linear model coefficient shape is invalid",
			nil,
		))
	}

	if len(row) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal: prediction row is empty",
			nil,
		))
	}

	prediction := model.coefficients[0]

	for predictorIndex, node := range model.predictors {
		if node < 0 || node >= len(row) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				fmt.Sprintf("causal: predictor node %d outside row width %d", node, len(row)),
				nil,
			))
		}

		value := row[node]

		if node == overrideNode {
			value = overrideValue
		}

		prediction += model.coefficients[predictorIndex+1] * value
	}

	return prediction, nil
}

func (model linearModel) counterfactualUplift(
	row []float64, treatment int, intervention float64,
) (float64, error) {
	observed, err := model.predict(row, -1, 0)

	if err != nil {
		return 0, err
	}

	counterfactual, err := model.predict(row, treatment, intervention)

	if err != nil {
		return 0, err
	}

	return counterfactual - observed, nil
}

func (nodeTable nodeTable) column(node int) ([]float64, error) {
	if err := nodeTable.validateNode(node); err != nil {
		return nil, err
	}

	values := make([]float64, len(nodeTable.rows))

	for rowIndex, row := range nodeTable.rows {
		values[rowIndex] = row[node]
	}

	return values, nil
}

func (nodeTable nodeTable) columns(nodes ...int) ([][]float64, error) {
	if err := nodeTable.validateNodes(nodes...); err != nil {
		return nil, err
	}

	columns := make([][]float64, 0, len(nodes))

	for _, node := range nodes {
		column, err := nodeTable.column(node)

		if err != nil {
			return nil, err
		}

		columns = append(columns, column)
	}

	return columns, nil
}

func (nodeTable nodeTable) validateNodes(nodes ...int) error {
	for _, node := range nodes {
		if err := nodeTable.validateNode(node); err != nil {
			return err
		}
	}

	return nil
}

func (nodeTable nodeTable) validateNode(node int) error {
	if len(nodeTable.rows) == 0 {
		return errnie.Error(errnie.Err(
			errnie.Validation,
			"causal: table is empty",
			nil,
		))
	}

	width := len(nodeTable.rows[0])

	if node < 0 || node >= width {
		return errnie.Error(errnie.Err(
			errnie.Validation,
			fmt.Sprintf("causal: node %d outside row width %d", node, width),
			nil,
		))
	}

	return nil
}

func deriveBandwidth(rows [][]float64, treatmentNode int) (float64, error) {
	if treatmentNode < 0 || len(rows) == 0 || treatmentNode >= len(rows[0]) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal: treatment node outside table width",
			nil,
		))
	}

	minSamples := max(2, int(math.Ceil(math.Sqrt(float64(len(rows))))))

	if len(rows) < minSamples {
		return 0, io.EOF
	}

	values := make([]float64, len(rows))

	for index := range rows {
		values[index] = rows[index][treatmentNode]

		if math.IsNaN(values[index]) || math.IsInf(values[index], 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"causal: treatment value is non-finite",
				nil,
			))
		}
	}

	variance := stat.Variance(values, nil)

	if variance <= 0 || math.IsNaN(variance) || math.IsInf(variance, 0) {
		return 0, io.EOF
	}

	sampleCount := float64(len(values))
	covariateDimension := float64(len(rows[0]) - 1)
	spread := math.Sqrt(variance)
	scale := robustScale(values)

	if covariateDimension < 1 {
		covariateDimension = 1
	}

	if scale <= 0 {
		scale = spread
	}

	relativeSpread := spread / scale
	smoothnessPenalty := math.Log(math.E + relativeSpread*relativeSpread*sampleCount)
	denominator := covariateDimension + smoothnessPenalty
	exponent := -1.0 / denominator

	return spread * math.Pow(sampleCount, exponent), nil
}

func intSlice(values []float64) []int {
	out := make([]int, len(values))

	for index, value := range values {
		out[index] = int(value)
	}

	return out
}

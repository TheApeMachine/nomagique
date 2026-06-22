package causal

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/statistic"
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
		return nodeTable{}, errors.New("causal: minRows must be positive")
	}

	if len(rows) < minRows {
		return nodeTable{}, fmt.Errorf("causal: table needs %d rows, got %d", minRows, len(rows))
	}

	nodeCount := len(rows[0])

	if nodeCount == 0 {
		return nodeTable{}, errors.New("causal: rows must contain nodes")
	}

	if target < 0 || target >= nodeCount {
		return nodeTable{}, fmt.Errorf("causal: target node %d outside row width %d", target, nodeCount)
	}

	for rowIndex, row := range rows {
		if len(row) != nodeCount {
			return nodeTable{}, fmt.Errorf(
				"causal: row %d width %d differs from %d",
				rowIndex, len(row), nodeCount,
			)
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

	association := stat.Correlation(treatmentValues, targetValues, nil)

	if math.IsNaN(association) || math.IsInf(association, 0) {
		return 0, errors.New("causal: association is non-finite")
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

	residualTarget, ok := statistic.Residualize(targetValues, controlColumns...)

	if !ok {
		return 0, errors.New("causal: target residualization failed")
	}

	residualTreatment, ok := statistic.Residualize(treatmentValues, controlColumns...)

	if !ok {
		return 0, errors.New("causal: treatment residualization failed")
	}

	denominator, denominatorOK := statistic.BackdoorDenominator(
		statistic.Dot(residualTreatment, residualTreatment),
	)

	if !denominatorOK {
		return 0, errors.New("causal: backdoor denominator is non-positive")
	}

	return statistic.Dot(residualTarget, residualTreatment) / denominator, nil
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

	coefficients, ok := statistic.OLS(targetValues, predictorColumns...)

	if !ok {
		return linearModel{}, errors.New("causal: linear structural fit failed")
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

	condition, ok := statistic.PairConditionNumber(leftColumn, rightColumn)

	if !ok {
		return 0, errors.New("causal: pair condition number failed")
	}

	return condition, nil
}

func (nodeTable nodeTable) percentile(node int, percentile float64) (float64, error) {
	values, err := nodeTable.column(node)

	if err != nil {
		return 0, err
	}

	if len(values) == 0 {
		return 0, errors.New("causal: percentile node has no values")
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	return stat.Quantile(percentile, stat.LinInterp, sorted, nil), nil
}

func (nodeTable nodeTable) kernelBackdoorEffect(
	treatment int, bandwidth float64, controls ...int,
) (float64, error) {
	if bandwidth <= 0 {
		return 0, errors.New("causal: kernel bandwidth must be positive")
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

	residualTarget, ok := statistic.Residualize(targetValues, controlColumns...)

	if !ok {
		return 0, errors.New("causal: target residualization failed")
	}

	residualTreatment, ok := statistic.Residualize(treatmentValues, controlColumns...)

	if !ok {
		return 0, errors.New("causal: treatment residualization failed")
	}

	current := nodeTable.rows[len(nodeTable.rows)-1]
	numerator := 0.0
	denominator := 0.0

	for rowIndex, row := range nodeTable.rows {
		distance := nodeDistance(current, row, controls)
		weight := math.Exp(-distance * distance / (2 * bandwidth * bandwidth))

		if weight <= 0 || math.IsNaN(weight) || math.IsInf(weight, 0) {
			continue
		}

		treatmentValue := residualTreatment[rowIndex]
		numerator += weight * residualTarget[rowIndex] * treatmentValue
		denominator += weight * treatmentValue * treatmentValue
	}

	denominator, denominatorOK := statistic.BackdoorDenominator(denominator)

	if !denominatorOK {
		return 0, errors.New("causal: backdoor denominator is non-positive")
	}

	return numerator / denominator, nil
}

func (model linearModel) predict(
	row []float64, overrideNode int, overrideValue float64,
) (float64, error) {
	if len(model.coefficients) != len(model.predictors)+1 {
		return 0, errors.New("causal: linear model coefficient shape is invalid")
	}

	if len(row) == 0 {
		return 0, errors.New("causal: prediction row is empty")
	}

	prediction := model.coefficients[0]

	for predictorIndex, node := range model.predictors {
		if node < 0 || node >= len(row) {
			return 0, fmt.Errorf("causal: predictor node %d outside row width %d", node, len(row))
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
		return errors.New("causal: table is empty")
	}

	width := len(nodeTable.rows[0])

	if node < 0 || node >= width {
		return fmt.Errorf("causal: node %d outside row width %d", node, width)
	}

	return nil
}

func nodeDistance(left, right []float64, features []int) float64 {
	sum := 0.0

	for _, feature := range features {
		delta := left[feature] - right[feature]
		sum += delta * delta
	}

	return math.Sqrt(sum)
}

func tableRows(artifact *datura.Artifact) ([][]float64, bool) {
	rowCount := int(datura.Peek[float64](artifact, "table", "rowCount"))
	nodeCount := int(datura.Peek[float64](artifact, "table", "nodeCount"))
	flat := datura.Peek[[]float64](artifact, "table", "rows")

	if rowCount <= 0 || nodeCount <= 0 || len(flat) != rowCount*nodeCount {
		return nil, false
	}

	rows := make([][]float64, rowCount)

	for rowIndex := range rows {
		rows[rowIndex] = make([]float64, nodeCount)
		offset := rowIndex * nodeCount

		for nodeIndex := range nodeCount {
			rows[rowIndex][nodeIndex] = flat[offset+nodeIndex]
		}
	}

	return rows, true
}

func deriveBandwidth(rows [][]float64, treatmentNode int) float64 {
	if treatmentNode < 0 || len(rows) == 0 || treatmentNode >= len(rows[0]) {
		return 0
	}

	if len(rows) < 12 {
		return 0
	}

	values := make([]float64, len(rows))

	for index := range rows {
		values[index] = rows[index][treatmentNode]
	}

	mean := 0.0

	for _, value := range values {
		mean += value
	}

	mean /= float64(len(values))

	variance := 0.0

	for _, value := range values {
		delta := value - mean
		variance += delta * delta
	}

	if len(values) > 1 {
		variance /= float64(len(values) - 1)
	}

	if variance <= 0 {
		return 0
	}

	return 1.06 * math.Sqrt(variance) * math.Pow(float64(len(values)), -0.2)
}

func intSlice(values []float64) []int {
	out := make([]int, len(values))

	for index, value := range values {
		out[index] = int(value)
	}

	return out
}

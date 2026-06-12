package causal

import (
	"errors"
	"fmt"
	"math"

	"github.com/theapemachine/nomagique/statistic"
	"gonum.org/v1/gonum/stat"
)

const minKernelWeight = 1e-9

/*
NodeTable is a tabular structural causal model over aligned observation rows.
*/
type NodeTable struct {
	rows   [][]float64
	target int
}

/*
LinearModel holds OLS coefficients for a target predicted from selected nodes.
*/
type LinearModel struct {
	coefficients []float64
	predictors   []int
}

/*
NewNodeTable validates and constructs a node table.
*/
func NewNodeTable(rows [][]float64, target, minRows int) (NodeTable, error) {
	if minRows <= 0 {
		return NodeTable{}, errors.New("causal: minRows must be positive")
	}

	if len(rows) < minRows {
		return NodeTable{}, fmt.Errorf("causal: table needs %d rows, got %d", minRows, len(rows))
	}

	nodeCount := len(rows[0])

	if nodeCount == 0 {
		return NodeTable{}, errors.New("causal: rows must contain nodes")
	}

	if target < 0 || target >= nodeCount {
		return NodeTable{}, fmt.Errorf("causal: target node %d outside row width %d", target, nodeCount)
	}

	for rowIndex, row := range rows {
		if len(row) != nodeCount {
			return NodeTable{}, fmt.Errorf(
				"causal: row %d width %d differs from %d",
				rowIndex, len(row), nodeCount,
			)
		}
	}

	return NodeTable{rows: rows, target: target}, nil
}

/*
Association returns Pearson correlation between treatment and target.
*/
func (nodeTable NodeTable) Association(treatment int) (float64, error) {
	treatmentValues, err := nodeTable.column(treatment)

	if err != nil {
		return 0, err
	}

	targetValues, err := nodeTable.column(nodeTable.target)

	if err != nil {
		return 0, err
	}

	return stat.Correlation(treatmentValues, targetValues, nil), nil
}

/*
BackdoorEffect estimates a linear backdoor adjustment via residualization.
*/
func (nodeTable NodeTable) BackdoorEffect(treatment int, controls ...int) (float64, error) {
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

	denominator := statistic.BackdoorDenominator(
		statistic.Dot(residualTreatment, residualTreatment),
	)

	return statistic.Dot(residualTarget, residualTreatment) / denominator, nil
}

/*
LinearModel fits target from predictors.
*/
func (nodeTable NodeTable) LinearModel(predictors ...int) (LinearModel, error) {
	targetValues, err := nodeTable.column(nodeTable.target)

	if err != nil {
		return LinearModel{}, err
	}

	predictorColumns, err := nodeTable.columns(predictors...)

	if err != nil {
		return LinearModel{}, err
	}

	coefficients, ok := statistic.OLS(targetValues, predictorColumns...)

	if !ok {
		return LinearModel{}, errors.New("causal: linear structural fit failed")
	}

	return LinearModel{
		coefficients: coefficients,
		predictors:   append([]int(nil), predictors...),
	}, nil
}

/*
PairConditionNumber returns the 2×2 correlation condition number for two nodes.
*/
func (nodeTable NodeTable) PairConditionNumber(left, right int) (float64, error) {
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

/*
Percentile returns a percentile of one node's history.
*/
func (nodeTable NodeTable) Percentile(node int, percentile float64) (float64, error) {
	values, err := nodeTable.column(node)

	if err != nil {
		return 0, err
	}

	if len(values) == 0 {
		return 0, errors.New("causal: percentile node has no values")
	}

	sorted := append([]float64(nil), values...)
	return percentileSorted(sorted, percentile), nil
}

/*
KernelBackdoorEffect estimates a Gaussian-kernel-weighted backdoor effect.
*/
func (nodeTable NodeTable) KernelBackdoorEffect(
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

	features := append([]int(nil), controls...)
	features = append(features, treatment)
	current := nodeTable.rows[len(nodeTable.rows)-1]
	numerator := 0.0
	denominator := 0.0

	for _, row := range nodeTable.rows {
		distance := nodeDistance(current, row, features)
		weight := math.Exp(-distance * distance / (2 * bandwidth * bandwidth))

		if weight < minKernelWeight {
			continue
		}

		treatmentValue := row[treatment]
		numerator += weight * row[nodeTable.target] * treatmentValue
		denominator += weight * treatmentValue * treatmentValue
	}

	denominator = statistic.BackdoorDenominator(denominator)

	return numerator / denominator, nil
}

/*
Predict returns a linear model prediction with optional treatment override.
*/
func (model LinearModel) Predict(
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

/*
CounterfactualUplift returns predicted change under a treatment intervention.
*/
func (model LinearModel) CounterfactualUplift(
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

func (nodeTable NodeTable) column(node int) ([]float64, error) {
	if err := nodeTable.validateNode(node); err != nil {
		return nil, err
	}

	values := make([]float64, len(nodeTable.rows))

	for rowIndex, row := range nodeTable.rows {
		values[rowIndex] = row[node]
	}

	return values, nil
}

func (nodeTable NodeTable) columns(nodes ...int) ([][]float64, error) {
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

func (nodeTable NodeTable) validateNodes(nodes ...int) error {
	for _, node := range nodes {
		if err := nodeTable.validateNode(node); err != nil {
			return err
		}
	}

	return nil
}

func (nodeTable NodeTable) validateNode(node int) error {
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

func percentileSorted(sorted []float64, percentile float64) float64 {
	if len(sorted) == 0 {
		return 0
	}

	index := percentile * float64(len(sorted)-1)
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))

	if lower == upper {
		return sorted[lower]
	}

	weight := index - float64(lower)

	return sorted[lower]*(1-weight) + sorted[upper]*weight
}

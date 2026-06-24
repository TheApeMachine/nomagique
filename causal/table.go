package causal

import (
	"fmt"
	"math"
	"sort"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
Table validates tabular wire payloads and exposes causal table operations on Read.
*/
type Table struct {
	artifact *datura.Artifact
}

/*
NewTable returns a table stage wired from config attributes on the artifact.
*/
func NewTable(artifact *datura.Artifact) *Table {
	return &Table{
		artifact: artifact,
	}
}

func (table *Table) Read(payload []byte) (int, error) {
	state := datura.Acquire("table-state", datura.APPJSON)

	if _, err := state.Write(table.artifact.DecryptPayload()); err != nil {
		state.Release()

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal table: state write failed",
			err,
		))
	}

	defer state.Release()

	rows, err := tableRows(state)

	if err != nil {
		return 0, err
	}

	operation := datura.Peek[string](table.artifact, "operation")

	if operation == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal table: operation required",
			nil,
		))
	}

	target := int(datura.Peek[float64](table.artifact, "target"))
	minRows := int(datura.Peek[float64](table.artifact, "minHistory"))

	if minRows <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal table: minHistory required",
			nil,
		))
	}

	nodeTable, err := newNodeTable(rows, target, minRows)

	if err != nil {
		return 0, err
	}

	switch operation {
	case "bandwidth":
		treatmentNode := int(datura.Peek[float64](table.artifact, "treatment"))

		bandwidth, bandwidthErr := deriveBandwidth(rows, treatmentNode)

		if bandwidthErr != nil {
			return 0, bandwidthErr
		}

		state.MergeOutput("value", bandwidth)

	case "association":
		treatment := int(datura.Peek[float64](table.artifact, "treatment"))

		value, associationErr := nodeTable.association(treatment)

		if associationErr != nil {
			return 0, associationErr
		}

		state.MergeOutput("value", value)

	case "backdoor":
		treatment := int(datura.Peek[float64](table.artifact, "treatment"))
		controls := intSlice(datura.Peek[[]float64](table.artifact, "controls"))

		value, backdoorErr := nodeTable.backdoorEffect(treatment, controls...)

		if backdoorErr != nil {
			return 0, backdoorErr
		}

		state.MergeOutput("value", value)

	case "kernelBackdoor":
		treatment := int(datura.Peek[float64](table.artifact, "treatment"))
		controls := intSlice(datura.Peek[[]float64](table.artifact, "controls"))
		bandwidth := datura.Peek[float64](table.artifact, "kernelBandwidth")

		if bandwidth <= 0 {
			derived, bandwidthErr := deriveBandwidth(rows, treatment)

			if bandwidthErr != nil {
				return 0, bandwidthErr
			}

			bandwidth = derived
		}

		value, kernelErr := nodeTable.kernelBackdoorEffect(treatment, bandwidth, controls...)

		if kernelErr != nil {
			return 0, kernelErr
		}

		state.MergeOutput("value", value)

	case "pairCondition":
		left := int(datura.Peek[float64](table.artifact, "left"))
		right := int(datura.Peek[float64](table.artifact, "right"))

		value, conditionErr := nodeTable.pairConditionNumber(left, right)

		if conditionErr != nil {
			return 0, conditionErr
		}

		state.MergeOutput("value", value)

	case "percentile":
		node := int(datura.Peek[float64](table.artifact, "node"))
		percentile := datura.Peek[float64](table.artifact, "percentile")

		value, percentileErr := nodeTable.percentile(node, percentile)

		if percentileErr != nil {
			return 0, percentileErr
		}

		state.MergeOutput("value", value)

	default:
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			fmt.Sprintf("causal table: unsupported operation %q", operation),
			nil,
		))
	}

	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")

	return state.Read(payload)
}

func (table *Table) Write(payload []byte) (int, error) {
	table.artifact.WithPayload(payload)
	return len(payload), nil
}

func (table *Table) Close() error {
	return nil
}

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

	association := stat.Correlation(treatmentValues, targetValues, nil)

	if math.IsNaN(association) || math.IsInf(association, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal: association is non-finite",
			nil,
		))
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

		if weight <= 0 || math.IsNaN(weight) || math.IsInf(weight, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"causal: kernel weight is non-positive or non-finite",
				nil,
			))
		}

		treatmentValue := residualTreatment[rowIndex]
		numerator += weight * residualTarget[rowIndex] * treatmentValue
		weightSum += weight * treatmentValue * treatmentValue
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

func tableRows(artifact *datura.Artifact) ([][]float64, error) {
	rowCount := int(datura.Peek[float64](artifact, "table", "rowCount"))
	nodeCount := int(datura.Peek[float64](artifact, "table", "nodeCount"))
	flat := datura.Peek[[]float64](artifact, "table", "rows")

	if rowCount <= 0 || nodeCount <= 0 || len(flat) != rowCount*nodeCount {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal: table rows invalid",
			nil,
		))
	}

	rows := make([][]float64, rowCount)

	for rowIndex := range rows {
		rows[rowIndex] = make([]float64, nodeCount)
		offset := rowIndex * nodeCount

		for nodeIndex := range nodeCount {
			rows[rowIndex][nodeIndex] = flat[offset+nodeIndex]
		}
	}

	return rows, nil
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
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal: insufficient rows for bandwidth derivation",
			nil,
		))
	}

	values := make([]float64, len(rows))

	for index := range rows {
		values[index] = rows[index][treatmentNode]
	}

	mean := stat.Mean(values, nil)

	if mean <= 0 || math.IsNaN(mean) || math.IsInf(mean, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal: treatment mean is non-positive or non-finite",
			nil,
		))
	}

	variance := stat.Variance(values, nil)

	if variance <= 0 || math.IsNaN(variance) || math.IsInf(variance, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal: treatment variance is non-positive or non-finite",
			nil,
		))
	}

	sampleCount := float64(len(values))
	covariateDimension := float64(len(rows[0]) - 1)

	if covariateDimension < 1 {
		covariateDimension = 1
	}

	relativeSpread := math.Sqrt(variance) / math.Abs(mean)
	smoothnessPenalty := math.Log(math.E + relativeSpread*relativeSpread*sampleCount)
	denominator := covariateDimension + smoothnessPenalty
	exponent := -1.0 / denominator

	return math.Sqrt(variance) * math.Pow(sampleCount, exponent), nil
}

func intSlice(values []float64) []int {
	out := make([]int, len(values))

	for index, value := range values {
		out[index] = int(value)
	}

	return out
}

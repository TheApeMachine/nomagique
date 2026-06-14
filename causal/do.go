package causal

/*
DoExpectation estimates E[Y|do(X=level)] via the g-formula over observed covariates.
*/
func (nodeTable NodeTable) DoExpectation(
	treatment int, level float64, controls ...int,
) (float64, error) {
	predictors := append(append([]int(nil), controls...), treatment)
	model, err := nodeTable.LinearModel(predictors...)

	if err != nil {
		return 0, err
	}

	total := 0.0

	for _, row := range nodeTable.rows {
		prediction, predErr := model.Predict(row, treatment, level)

		if predErr != nil {
			return 0, predErr
		}

		total += prediction
	}

	return total / float64(len(nodeTable.rows)), nil
}

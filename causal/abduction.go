package causal

import "errors"

/*
AbductNoise infers exogenous residual U = Y - f(X, Z) for one observed row.
*/
func (model LinearModel) AbductNoise(row []float64, observedTarget float64) (float64, error) {
	fitted, err := model.Predict(row, -1, 0)

	if err != nil {
		return 0, err
	}

	return observedTarget - fitted, nil
}

/*
StructuralCounterfactual returns Y_{x'}(u) = f(x', Z) + U after abduction.
*/
func (model LinearModel) StructuralCounterfactual(
	row []float64,
	treatment int,
	intervention float64,
	noise float64,
) (float64, error) {
	structural, err := model.Predict(row, treatment, intervention)

	if err != nil {
		return 0, err
	}

	return structural + noise, nil
}

/*
AbductNoise infers exogenous residual U = Y - f(X, Z) for one observed row.
*/
func (model NonLinearModel) AbductNoise(row []float64, observedTarget float64) (float64, error) {
	fitted, err := model.Predict(row, -1, 0)

	if err != nil {
		return 0, err
	}

	return observedTarget - fitted, nil
}

/*
StructuralCounterfactual returns Y_{x'}(u) = f(x', Z) + U after abduction.
*/
func (model NonLinearModel) StructuralCounterfactual(
	row []float64,
	treatment int,
	intervention float64,
	noise float64,
) (float64, error) {
	structural, err := model.Predict(row, treatment, intervention)

	if err != nil {
		return 0, err
	}

	return structural + noise, nil
}

/*
AbductiveCounterfactual runs abduction then counterfactual intervention on one row.
*/
func AbductiveCounterfactual(
	model NonLinearModel,
	row []float64,
	target int,
	treatment int,
	intervention float64,
) (uplift, counterfactual, noise float64, err error) {
	if target < 0 || target >= len(row) {
		return 0, 0, 0, errors.New("causal: abduction target outside row")
	}

	observedTarget := row[target]
	noise, err = model.AbductNoise(row, observedTarget)

	if err != nil {
		return 0, 0, 0, err
	}

	counterfactual, err = model.StructuralCounterfactual(row, treatment, intervention, noise)

	if err != nil {
		return 0, 0, 0, err
	}

	return counterfactual - observedTarget, counterfactual, noise, nil
}

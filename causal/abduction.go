package causal

import (
	"errors"
)

// NodeTable wraps the package-private nodeTable
/*
Transitive causal layer from symm — NodeTable estimates do/backdoor effects on tabular desk features.
*/
type NodeTable struct {
/*
Transitive causal layer from symm — NewNodeTableWrapper estimates do/backdoor effects on tabular desk features.
*/
	Nt nodeTable
}

/*
Transitive causal layer from symm — NewNodeTableWrapper estimates do/backdoor and counterfactual effects on desk tables.
*/

func NewNodeTableWrapper(rows [][]float64, target int, _ ...int) (NodeTable, error) {
	nt, err := newNodeTable(rows, target)
	if err != nil {
		return NodeTable{}, err
	}
	return NodeTable{Nt: nt}, nil
}

/*
Transitive causal layer from symm — DoExpectation estimates do/backdoor and counterfactual effects on desk tables.
*/

func (wrapper NodeTable) DoExpectation(treatment int, level float64, controls ...int) (float64, error) {
	return doExpectation(wrapper.Nt, treatment, level, controls...)
}
/*
Transitive causal layer from symm — abductiveCounterfactual estimates do/backdoor effects on tabular desk features.
*/

func (wrapper NodeTable) AbductiveCounterfactual(
	features []int,
	linear bool,
	row []float64,
	target int,
	treatment int,
	intervention float64,
) (uplift, counterfactual, noise float64, err error) {
	return abductiveCounterfactual(wrapper.Nt, features, linear, row, target, treatment, intervention)
}

/*
Transitive causal layer from symm — abductiveCounterfactual estimates do/backdoor and counterfactual effects on desk tables.
*/

func abductiveCounterfactual(
	table nodeTable,
	features []int,
	linear bool,
	row []float64,
	target int,
	treatment int,
	intervention float64,
) (uplift, counterfactual, noise float64, err error) {
	if target < 0 || target >= len(row) {
		return 0, 0, 0, errors.New("causal: abduction target outside row")
	}

	observedTarget := row[target]

	if linear {
		model, err := table.fitLinearModel(features...)

		if err != nil {
			return 0, 0, 0, err
		}

		noise, err = abductNoiseLinear(model, row, observedTarget)

		if err != nil {
			return 0, 0, 0, err
		}

		counterfactual, err = structuralCounterfactualLinear(model, row, treatment, intervention, noise)

		if err != nil {
			return 0, 0, 0, err
		}

		return counterfactual - observedTarget, counterfactual, noise, nil
	}

	model, fitOK := fitNonLinearTable(table, features)

	if !fitOK {
		return 0, 0, 0, errors.New("causal: nonlinear abduction fit failed")
	}

	noise, err = abductNoiseNonLinear(model, row, observedTarget)

	if err != nil {
		return 0, 0, 0, err
	}

	counterfactual, err = structuralCounterfactualNonLinear(model, row, treatment, intervention, noise)
	if err != nil {
		return 0, 0, 0, err
	}

	return counterfactual - observedTarget, counterfactual, noise, nil
}

/*
Transitive causal layer from symm — abductNoiseLinear estimates do/backdoor and counterfactual effects on desk tables.
*/

func abductNoiseLinear(model linearModel, row []float64, observedTarget float64) (float64, error) {
	fitted, err := model.predict(row, -1, 0)

	if err != nil {
		return 0, err
	}

	return observedTarget - fitted, nil
}

/*
Transitive causal layer from symm — structuralCounterfactualLinear estimates do/backdoor and counterfactual effects on desk tables.
*/

func structuralCounterfactualLinear(
	model linearModel,
	row []float64,
	treatment int,
	intervention float64,
	noise float64,
) (float64, error) {
	structural, err := model.predict(row, treatment, intervention)

	if err != nil {
		return 0, err
	}

	return structural + noise, nil
}

/*
Transitive causal layer from symm — abductNoiseNonLinear estimates do/backdoor and counterfactual effects on desk tables.
*/

func abductNoiseNonLinear(model nonLinearModel, row []float64, observedTarget float64) (float64, error) {
	fitted, err := model.predict(row, -1, 0)

	if err != nil {
		return 0, err
	}

	return observedTarget - fitted, nil
}

/*
Transitive causal layer from symm — structuralCounterfactualNonLinear estimates do/backdoor and counterfactual effects on desk tables.
*/

func structuralCounterfactualNonLinear(
	model nonLinearModel,
	row []float64,
	treatment int,
	intervention float64,
	noise float64,
) (float64, error) {
	structural, err := model.predict(row, treatment, intervention)

	if err != nil {
		return 0, err
	}

	return structural + noise, nil
}

package causal

// MCTSAdapter implements the CausalEngine interface using internal causal package structures.
type MCTSAdapter struct{}

func NewMCTSAdapter() *MCTSAdapter {
	return &MCTSAdapter{}
}

func (a *MCTSAdapter) DoExpectation(
	rows [][]float64,
	target, minRows, treatment int,
	level float64,
	controls []int,
) (float64, error) {
	table, err := newNodeTable(rows, target, minRows)
	if err != nil {
		return 0, err
	}
	return doExpectation(table, treatment, level, controls...)
}

func (a *MCTSAdapter) AbductiveCounterfactual(
	rows [][]float64,
	target, minRows int,
	features []int,
	linear bool,
	row []float64,
	treatment int,
	intervention float64,
) (float64, float64, error) {
	table, err := newNodeTable(rows, target, minRows)
	if err != nil {
		return 0, 0, err
	}

	// Returns uplift, counterfactual, noise, err
	_, counterfactual, noise, err := abductiveCounterfactual(
		table,
		features,
		linear,
		row,
		target,
		treatment,
		intervention,
	)
	return counterfactual, noise, err
}

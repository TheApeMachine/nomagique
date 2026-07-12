package hawkes

import "math"

/*
ImmediateOffspring returns the expected first-generation children caused by one
buy parent and one sell parent. It uses column sums of the branching matrix
K=A/beta because columns identify the parent stream.
*/
func (fit BivariateFit) ImmediateOffspring() (
	buyParent float64,
	sellParent float64,
	ok bool,
) {
	if !fit.Valid() {
		return 0, 0, false
	}

	buyParent = (fit.AlphaXX + fit.AlphaYX) / fit.Beta
	sellParent = (fit.AlphaXY + fit.AlphaYY) / fit.Beta

	if !finiteNonNegative(buyParent) || !finiteNonNegative(sellParent) {
		return 0, 0, false
	}

	return buyParent, sellParent, true
}

/*
TotalDescendants returns all expected descendants across every generation for
one parent on each side. For a stable process this is the column sum of
(I-K)^-1 minus the original parent.
*/
func (fit BivariateFit) TotalDescendants() (
	buyParent float64,
	sellParent float64,
	ok bool,
) {
	if !fit.Valid() {
		return 0, 0, false
	}

	branch := fit.Params().branchingMatrix()
	determinant := (1-branch[0][0])*(1-branch[1][1]) -
		branch[0][1]*branch[1][0]

	if determinant <= 0 {
		return 0, 0, false
	}

	buyParent = (1-branch[1][1]+branch[1][0])/determinant - 1
	sellParent = (branch[0][1]+1-branch[0][0])/determinant - 1

	if !finiteNonNegative(buyParent) || !finiteNonNegative(sellParent) {
		return 0, 0, false
	}

	return buyParent, sellParent, true
}

func finiteNonNegative(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

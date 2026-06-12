package hawkes

import "math"

/*
BivariateParams holds exponential-kernel Hawkes parameters for buy and sell streams.
*/
type BivariateParams struct {
	MuBuy   float64
	MuSell  float64
	AlphaBB float64
	AlphaBS float64
	AlphaSB float64
	AlphaSS float64
	Beta    float64
}

/*
MeanIntensity returns stationary mean intensities under the branching matrix G = A / beta.
*/
func (params BivariateParams) MeanIntensity() (buy float64, sell float64, ok bool) {
	if params.Beta <= 0 {
		return 0, 0, false
	}

	branching := params.branchingMatrix()
	determinant := (1-branching[0][0])*(1-branching[1][1]) - branching[0][1]*branching[1][0]

	if determinant <= 0 {
		return 0, 0, false
	}

	buy = ((1-branching[1][1])*params.MuBuy + branching[0][1]*params.MuSell) / determinant
	sell = (branching[1][0]*params.MuBuy + (1-branching[0][0])*params.MuSell) / determinant

	if buy < 0 || sell < 0 || math.IsNaN(buy) || math.IsNaN(sell) {
		return 0, 0, false
	}

	return buy, sell, true
}

/*
Stable reports whether the branching spectral radius stays below one.
*/
func (params BivariateParams) Stable() bool {
	if params.Beta <= 0 {
		return false
	}

	return spectralRadius(params.branchingMatrix()) < 1
}

func (params BivariateParams) branchingMatrix() [2][2]float64 {
	invBeta := 1 / params.Beta

	return [2][2]float64{
		{params.AlphaBB * invBeta, params.AlphaBS * invBeta},
		{params.AlphaSB * invBeta, params.AlphaSS * invBeta},
	}
}

func spectralRadius(matrix [2][2]float64) float64 {
	trace := matrix[0][0] + matrix[1][1]
	determinant := matrix[0][0]*matrix[1][1] - matrix[0][1]*matrix[1][0]

	discriminant := trace*trace/4 - determinant

	if discriminant < 0 {
		discriminant = 0
	}

	root := math.Sqrt(discriminant)
	first := trace/2 + root
	second := trace/2 - root

	return math.Max(first, second)
}

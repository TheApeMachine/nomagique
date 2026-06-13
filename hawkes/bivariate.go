package hawkes

import "math"

/*
BivariateParams holds exponential-kernel Hawkes parameters for two coupled streams x and y.
*/
type BivariateParams struct {
	MuX     float64
	MuY     float64
	AlphaXX float64
	AlphaXY float64
	AlphaYX float64
	AlphaYY float64
	Beta    float64
}

/*
MeanIntensity returns stationary mean intensities under the branching matrix G = A / beta.
*/
func (params BivariateParams) MeanIntensity() (lambdaX float64, lambdaY float64, ok bool) {
	if params.Beta <= 0 {
		return 0, 0, false
	}

	branching := params.branchingMatrix()
	determinant := (1-branching[0][0])*(1-branching[1][1]) - branching[0][1]*branching[1][0]

	if determinant <= 0 {
		return 0, 0, false
	}

	lambdaX = ((1-branching[1][1])*params.MuX + branching[0][1]*params.MuY) / determinant
	lambdaY = (branching[1][0]*params.MuX + (1-branching[0][0])*params.MuY) / determinant

	if lambdaX < 0 || lambdaY < 0 || math.IsNaN(lambdaX) || math.IsNaN(lambdaY) {
		return 0, 0, false
	}

	return lambdaX, lambdaY, true
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
		{params.AlphaXX * invBeta, params.AlphaXY * invBeta},
		{params.AlphaYX * invBeta, params.AlphaYY * invBeta},
	}
}

func spectralRadius(matrix [2][2]float64) float64 {
	return SpectralRadius(matrix)
}

/*
SpectralRadius returns the spectral radius of a 2×2 branching matrix.
Complex eigenvalues use modulus; real eigenvalues use maximum absolute value.
*/
func SpectralRadius(matrix [2][2]float64) float64 {
	trace := matrix[0][0] + matrix[1][1]
	determinant := matrix[0][0]*matrix[1][1] - matrix[0][1]*matrix[1][0]
	discriminant := trace*trace - 4*determinant

	if discriminant < 0 {
		modulus := math.Sqrt(-discriminant)
		realPart := trace / 2
		imagPart := modulus / 2

		return math.Sqrt(realPart*realPart + imagPart*imagPart)
	}

	rootHigh := (trace + math.Sqrt(discriminant)) / 2
	rootLow := (trace - math.Sqrt(discriminant)) / 2

	return math.Max(math.Abs(rootHigh), math.Abs(rootLow))
}

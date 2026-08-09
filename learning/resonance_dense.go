package learning

import (
	"math"

	"gonum.org/v1/gonum/mat"
)

func denseColDot(left, right *mat.Dense) float64 {
	return mat.Dot(
		left.ColView(0),
		right.ColView(0),
	)
}

func denseColNorm(matrix *mat.Dense) float64 {
	return mat.Norm(matrix.ColView(0), 2)
}

func denseApplyTanhInPlace(matrix *mat.Dense) {
	matrix.Apply(
		func(_, _ int, value float64) float64 {
			return math.Tanh(value)
		},
		matrix,
	)
}

func denseApplyOneMinusSquareInto(dst, src *mat.Dense) {
	dst.Apply(
		func(_, _ int, value float64) float64 {
			return 1.0 - value*value
		},
		src,
	)
}

func denseClipColInPlace(matrix *mat.Dense, clip float64) {
	matrix.Apply(
		func(_, _ int, value float64) float64 {
			switch {
			case value > clip:
				return clip
			case value < -clip:
				return -clip
			default:
				return value
			}
		},
		matrix,
	)
}

func denseOuterColsInto(
	dst *mat.Dense,
	left *mat.Dense,
	right *mat.Dense,
	scale float64,
) {
	dst.Outer(
		scale,
		left.ColView(0),
		right.ColView(0),
	)
}

func denseMulWeightTransposeInto(
	dst *mat.Dense,
	weight *mat.Dense,
	signal *mat.Dense,
) {
	dst.Mul(
		weight.T(),
		signal,
	)
}

func (rm *ResonanceManifold) constrainTemporalWeights() {
	if rm.A == nil {
		return
	}

	const maxNorm = 0.95

	norm := mat.Norm(rm.A, 2)
	if norm > maxNorm {
		rm.A.Scale(maxNorm/norm, rm.A)
	}
}

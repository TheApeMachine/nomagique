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

func denseFill(matrix *mat.Dense, value float64) {
	matrix.Apply(
		func(_, _ int, _ float64) float64 {
			return value
		},
		matrix,
	)
}

func denseVarianceEMAInto(
	variance *mat.Dense,
	residual *mat.Dense,
	scratch *mat.Dense,
	beta float64,
	floor float64,
) {
	scratch.MulElem(residual, residual)
	scratch.Scale(beta, scratch)
	variance.Scale(1.0-beta, variance)
	variance.Add(variance, scratch)
	variance.Apply(
		func(_, _ int, value float64) float64 {
			return math.Max(floor, value)
		},
		variance,
	)
}

func densePrecisionFromVarianceInto(
	precision *mat.Dense,
	variance *mat.Dense,
	minimum float64,
	maximum float64,
) {
	precision.Apply(
		func(_, _ int, varianceValue float64) float64 {
			return math.Min(maximum, math.Max(minimum, 1.0/varianceValue))
		},
		variance,
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

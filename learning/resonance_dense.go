package learning

import (
	"math"

	"gonum.org/v1/gonum/blas"
	"gonum.org/v1/gonum/blas/blas64"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
)

/*
denseColDot and denseColNorm wrap Gonum norms so the manifold hot path never
allocates or boxes slice headers per layer.
*/
func denseColDot(left, right *mat.VecDense) float64 {
	return floats.Dot(left.RawVector().Data, right.RawVector().Data)
}

/*
Learning package shared with symm resonance — denseColNorm supports predictive coding and online task heads.
*/
func denseColNorm(vector *mat.VecDense) float64 {
	return floats.Norm(vector.RawVector().Data, 2)
}

/*
denseApplyTanhInPlace keeps latent squashing in raw storage — the same layout
Metallib and CPU settle loops write into.
*/
func denseApplyTanhInPlace(vector *mat.VecDense) {
	data := vector.RawVector().Data

	for index, value := range data {
		data[index] = math.Tanh(value)
	}
}

/*
denseApplyOneMinusSquareInto forms (1−z²) for tanh derivative and precision
chains without re-reading src after dst is aliased.
*/
func denseApplyOneMinusSquareInto(dst, src *mat.VecDense) {
	dstData := dst.RawVector().Data
	srcData := src.RawVector().Data

	for index, value := range srcData {
		dstData[index] = 1.0 - value*value
	}
}

/*
denseFill seeds error variance and precision vectors at construction so the
first settle never divides by zero.
*/
func denseFill(vector *mat.VecDense, value float64) {
	data := vector.RawVector().Data

	for index := range data {
		data[index] = value
	}
}

/*
denseScaleInPlace applies learning-rate scalings directly on weight blocks.
*/
func denseScaleInPlace(matrix *mat.Dense, scale float64) {
	floats.Scale(scale, matrix.RawMatrix().Data)
}

/*
denseVarianceEMAInto tracks running reconstruction error variance per unit so
precision can track signal quality instead of staying at the initial guess.
*/
func denseVarianceEMAInto(
	variance *mat.VecDense,
	residual *mat.VecDense,
	beta float64,
	floor float64,
) {
	varianceData := variance.RawVector().Data
	residualData := residual.RawVector().Data
	retainedWeight := 1.0 - beta

	for index, residualValue := range residualData {
		varianceValue := retainedWeight*varianceData[index] +
			beta*(residualValue*residualValue)

		varianceData[index] = math.Max(floor, varianceValue)
	}
}

/*
densePrecisionFromVarianceInto inverts variance into a clipped precision so
state updates weight surprising errors more without exploding when variance
hits the EMA floor.
*/
func densePrecisionFromVarianceInto(
	precision *mat.VecDense,
	variance *mat.VecDense,
	minimum float64,
	maximum float64,
) {
	precisionData := precision.RawVector().Data
	varianceData := variance.RawVector().Data

	for index, varianceValue := range varianceData {
		precisionData[index] = math.Min(
			maximum,
			math.Max(minimum, 1.0/varianceValue),
		)
	}
}

/*
denseClipColInPlace enforces StateClip on latent columns during inference line
search without allocating a fresh vector each halving.
*/
func denseClipColInPlace(vector *mat.VecDense, clip float64) {
	data := vector.RawVector().Data

	for index, value := range data {
		switch {
		case value > clip:
			data[index] = clip
		case value < -clip:
			data[index] = -clip
		}
	}
}

/*
denseOuterColsInto forms scaled outer products for local Hebbian weight nudges.
*/
func denseOuterColsInto(
	dst *mat.Dense,
	left *mat.VecDense,
	right *mat.VecDense,
	scale float64,
) {
	dst.Outer(scale, left, right)
}

/*
denseMulWeightTransposeInto is Wᵀx into a pre-sized column — the recognition
pass in predictive coding.
*/
func denseMulWeightTransposeInto(
	dst *mat.VecDense,
	weight *mat.Dense,
	signal *mat.VecDense,
) {
	blas64.Gemv(
		blas.Trans,
		1,
		weight.RawMatrix(),
		signal.RawVector(),
		0,
		dst.RawVector(),
	)
}

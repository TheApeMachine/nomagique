package learning

import (
	"math"

	"gonum.org/v1/gonum/mat"
)

func denseColDot(left, right *mat.Dense) float64 {
	rows, _ := left.Dims()
	sum := 0.0

	for rowIndex := 0; rowIndex < rows; rowIndex++ {
		sum += left.At(rowIndex, 0) * right.At(rowIndex, 0)
	}

	return sum
}

func denseColNorm(matrix *mat.Dense) float64 {
	rows, _ := matrix.Dims()
	sum := 0.0

	for rowIndex := 0; rowIndex < rows; rowIndex++ {
		value := matrix.At(rowIndex, 0)
		sum += value * value
	}

	return math.Sqrt(sum)
}

func denseApplyTanhInPlace(matrix *mat.Dense) {
	rows, cols := matrix.Dims()
	raw := matrix.RawMatrix()

	for rowIndex := 0; rowIndex < rows; rowIndex++ {
		for colIndex := 0; colIndex < cols; colIndex++ {
			offset := rowIndex*raw.Stride + colIndex
			raw.Data[offset] = math.Tanh(raw.Data[offset])
		}
	}
}

func denseApplyOneMinusSquareInto(dst, src *mat.Dense) {
	rows, _ := src.Dims()

	for rowIndex := 0; rowIndex < rows; rowIndex++ {
		value := src.At(rowIndex, 0)
		dst.Set(rowIndex, 0, 1.0-value*value)
	}
}

func denseClipColInPlace(matrix *mat.Dense, clip float64) {
	rows, _ := matrix.Dims()

	for rowIndex := 0; rowIndex < rows; rowIndex++ {
		value := matrix.At(rowIndex, 0)

		if value > clip {
			matrix.Set(rowIndex, 0, clip)
			continue
		}

		if value < -clip {
			matrix.Set(rowIndex, 0, -clip)
		}
	}
}

func denseOuterColsInto(dst *mat.Dense, left, right *mat.Dense, scale float64) {
	leftRows, _ := left.Dims()
	rightRows, _ := right.Dims()
	dstRaw := dst.RawMatrix()

	for rowIndex := 0; rowIndex < leftRows; rowIndex++ {
		leftValue := left.At(rowIndex, 0) * scale

		for colIndex := 0; colIndex < rightRows; colIndex++ {
			dstRaw.Data[rowIndex*dstRaw.Stride+colIndex] = leftValue * right.At(colIndex, 0)
		}
	}
}

func denseMulWeightTransposeInto(dst, weight, signal *mat.Dense) {
	signalRows, weightCols := weight.Dims()

	for colIndex := 0; colIndex < weightCols; colIndex++ {
		sum := 0.0

		for rowIndex := 0; rowIndex < signalRows; rowIndex++ {
			sum += weight.At(rowIndex, colIndex) * signal.At(rowIndex, 0)
		}

		dst.Set(colIndex, 0, sum)
	}
}

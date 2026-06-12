package geometry

import (
	"fmt"

	"gonum.org/v1/gonum/mat"
)

/*
ProcrustesResult holds the outcome of an orthogonal Procrustes alignment
between two embedding spaces. R is the nDim×nDim rotation matrix that
minimizes ||R·A − B||_F², and Residual is the squared Frobenius norm of
the post-alignment error.
*/
type ProcrustesResult struct {
	R        *mat.Dense
	Residual float64
}

/*
Procrustes computes the orthogonal Procrustes alignment between sample matrices
A and B (both nSamples × nDim). Finds the rotation R minimizing ||R·A − B||²
via SVD of M = Bᵀ·A, then R = U·Vᵀ. A sign correction on the last column of U
enforces det(R) = +1 (proper rotation).
*/
func Procrustes(matA, matB mat.Matrix) (*ProcrustesResult, error) {
	nSamples, nDim := matA.Dims()
	rowsB, colsB := matB.Dims()

	if nSamples < 1 || nDim < 1 || rowsB != nSamples || colsB != nDim {
		return nil, ProcrustesError("dimension mismatch")
	}

	var bTranspose mat.Dense

	bTranspose.CloneFrom(matB.T())

	var cross mat.Dense

	cross.Mul(&bTranspose, matA)

	uMat, _, vMat, svdErr := JacobiSVD(&cross)

	if svdErr != nil {
		return nil, svdErr
	}

	var vTranspose mat.Dense

	vTranspose.CloneFrom(vMat.T())

	var rotation mat.Dense

	rotation.Mul(uMat, &vTranspose)

	var lu mat.LU

	lu.Factorize(&rotation)

	if lu.Det() < 0 {
		for rowIdx := 0; rowIdx < nDim; rowIdx++ {
			uMat.Set(rowIdx, nDim-1, -uMat.At(rowIdx, nDim-1))
		}

		rotation.Mul(uMat, &vTranspose)
	}

	var aTranspose mat.Dense

	aTranspose.CloneFrom(matA.T())

	var rotated mat.Dense

	rotated.Mul(&rotation, &aTranspose)

	var diff mat.Dense

	diff.Sub(&rotated, &bTranspose)

	residual := mat.Norm(&diff, 2)

	return &ProcrustesResult{
		R:        &rotation,
		Residual: residual * residual,
	}, nil
}

/*
JacobiSVD computes the thin SVD of an m×n matrix (m ≥ n). The name is
historical: the implementation uses gonum/mat.SVD rather than Jacobi sweeps.
Returns U (m×n), singular values Σ (length n, descending), and V (n×n).
*/
func JacobiSVD(matrix mat.Matrix) (*mat.Dense, []float64, *mat.Dense, error) {
	rows, cols := matrix.Dims()

	if rows < cols {
		return nil, nil, nil, ProcrustesError(fmt.Sprintf(
			"JacobiSVD requires rows ≥ cols, got %d × %d", rows, cols,
		))
	}

	var dense mat.Dense

	dense.CloneFrom(matrix)

	var svd mat.SVD

	if !svd.Factorize(&dense, mat.SVDThin) {
		return nil, nil, nil, ProcrustesError("SVD factorization failed")
	}

	sigma := svd.Values(nil)

	var uDense, vDense mat.Dense

	svd.UTo(&uDense)
	svd.VTo(&vDense)

	return &uDense, sigma, &vDense, nil
}

/*
DenseFromRows builds an nSamples × nDim matrix from a row-major slice-of-slices.
*/
func DenseFromRows(rows [][]float64, nSamples, nDim int) (*mat.Dense, error) {
	if len(rows) != nSamples {
		return nil, ProcrustesError("row count mismatch")
	}

	data := make([]float64, nSamples*nDim)

	for rowIdx := 0; rowIdx < nSamples; rowIdx++ {
		if len(rows[rowIdx]) != nDim {
			return nil, ProcrustesError("column count mismatch")
		}

		copy(data[rowIdx*nDim:(rowIdx+1)*nDim], rows[rowIdx])
	}

	return mat.NewDense(nSamples, nDim, data), nil
}

/*
ProcrustesError is a typed error for Procrustes and SVD failures.
*/
type ProcrustesError string

/*
Error implements the error interface for ProcrustesError.
*/
func (err ProcrustesError) Error() string {
	return string(err)
}

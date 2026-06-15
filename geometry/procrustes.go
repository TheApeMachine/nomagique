package geometry

import (
	"fmt"

	"github.com/theapemachine/datura"
	"gonum.org/v1/gonum/mat"
)

/*
ProcrustesResult holds the outcome of an orthogonal Procrustes alignment.
*/
type ProcrustesResult struct {
	R        *mat.Dense
	Residual float64
}

/*
Procrustes computes the orthogonal alignment between two sample matrices.
*/
type Procrustes struct {
	artifact *datura.Artifact
	matA     mat.Matrix
	matB     mat.Matrix
	result   ProcrustesResult
	err      error
	output   float64
}

func NewProcrustes(matA, matB mat.Matrix) *Procrustes {
	return &Procrustes{
		artifact: datura.Acquire("procrustes", datura.Artifact_Type_json),
		matA:     matA,
		matB:     matB,
	}
}

func NewProcrustesFromRows(
	rowsA, rowsB [][]float64,
	nSamples, nDim int,
) (*Procrustes, error) {
	denseA, err := denseFromRows(rowsA, nSamples, nDim)

	if err != nil {
		return nil, err
	}

	denseB, err := denseFromRows(rowsB, nSamples, nDim)

	if err != nil {
		return nil, err
	}

	return NewProcrustes(denseA, denseB), nil
}

func (procrustes *Procrustes) Write(p []byte) (int, error) {
	return procrustes.artifact.Write(p)
}

func (procrustes *Procrustes) Read(p []byte) (int, error) {
	procrustes.result, procrustes.err = procrustes.align(procrustes.matA, procrustes.matB)

	if procrustes.err != nil {
		procrustes.output = 0
		putFloat64Payload(&procrustes.artifact, "procrustes", procrustes.output)

		return procrustes.artifact.Read(p)
	}

	procrustes.output = procrustes.result.Residual
	putFloat64Payload(&procrustes.artifact, "procrustes", procrustes.output)

	return procrustes.artifact.Read(p)
}

func (procrustes *Procrustes) Close() error {
	return nil
}

func (procrustes *Procrustes) Result() ProcrustesResult {
	return procrustes.result
}

/*
Err returns the last alignment error.
*/
func (procrustes *Procrustes) Err() error {
	return procrustes.err
}

func (procrustes *Procrustes) Reset() error {
	procrustes.result = ProcrustesResult{}
	procrustes.err = nil
	procrustes.output = 0

	return nil
}

func (procrustes *Procrustes) Decompose(matrix mat.Matrix) (
	*mat.Dense, []float64, *mat.Dense, error,
) {
	return procrustes.jacobiSVD(matrix)
}

func (procrustes *Procrustes) align(matA, matB mat.Matrix) (ProcrustesResult, error) {
	nSamples, nDim := matA.Dims()
	rowsB, colsB := matB.Dims()

	if nSamples < 1 || nDim < 1 || rowsB != nSamples || colsB != nDim {
		return ProcrustesResult{}, procrustesError("dimension mismatch")
	}

	var bTranspose mat.Dense

	bTranspose.CloneFrom(matB.T())

	var cross mat.Dense

	cross.Mul(&bTranspose, matA)

	uMat, _, vMat, svdErr := procrustes.jacobiSVD(&cross)

	if svdErr != nil {
		return ProcrustesResult{}, svdErr
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

	return ProcrustesResult{
		R:        &rotation,
		Residual: residual * residual,
	}, nil
}

func (procrustes *Procrustes) jacobiSVD(matrix mat.Matrix) (
	*mat.Dense, []float64, *mat.Dense, error,
) {
	rows, cols := matrix.Dims()

	if rows < cols {
		return nil, nil, nil, procrustesError(fmt.Sprintf(
			"jacobiSVD requires rows ≥ cols, got %d × %d", rows, cols,
		))
	}

	var dense mat.Dense

	dense.CloneFrom(matrix)

	var svd mat.SVD

	if !svd.Factorize(&dense, mat.SVDThin) {
		return nil, nil, nil, procrustesError("SVD factorization failed")
	}

	sigma := svd.Values(nil)

	var uDense, vDense mat.Dense

	svd.UTo(&uDense)
	svd.VTo(&vDense)

	return &uDense, sigma, &vDense, nil
}

func denseFromRows(rows [][]float64, nSamples, nDim int) (*mat.Dense, error) {
	if len(rows) != nSamples {
		return nil, procrustesError("row count mismatch")
	}

	data := make([]float64, nSamples*nDim)

	for rowIdx := range nSamples {
		if len(rows[rowIdx]) != nDim {
			return nil, procrustesError("column count mismatch")
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

func procrustesError(message string) ProcrustesError {
	return ProcrustesError(message)
}

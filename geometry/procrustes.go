package geometry

import (
	"fmt"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
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
Matrix data is read from artifact attributes rowsA and rowsB, either as
[][]float64 or flat []float64 with nSamples and nDim.
*/
type Procrustes struct {
	artifact *datura.Artifact
}

func NewProcrustes(artifact *datura.Artifact) *Procrustes {
	return &Procrustes{
		artifact: artifact,
	}
}

func NewProcrustesFromRows(
	artifact *datura.Artifact,
	rowsA, rowsB [][]float64,
	nSamples, nDim int,
) (*Procrustes, error) {
	if len(rowsA) != nSamples || len(rowsB) != nSamples {
		return nil, procrustesError("row count mismatch")
	}

	for rowIdx := range nSamples {
		if len(rowsA[rowIdx]) != nDim || len(rowsB[rowIdx]) != nDim {
			return nil, procrustesError("column count mismatch")
		}
	}

	artifact.Poke(float64(nSamples), "nSamples")
	artifact.Poke(float64(nDim), "nDim")
	artifact.Poke(rowsToFlat(rowsA, nSamples, nDim), "rowsA")
	artifact.Poke(rowsToFlat(rowsB, nSamples, nDim), "rowsB")

	return NewProcrustes(artifact), nil
}

func (procrustes *Procrustes) Read(payload []byte) (int, error) {
	state := datura.Acquire("procrustes-state", datura.APPJSON)

	if _, err := state.Write(procrustes.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"procrustes: state write failed",
			err,
		))
	}

	matA, matB, err := matricesFromArtifact(procrustes.artifact)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"procrustes: matrix read failed",
			err,
		))
	}

	result, err := procrustes.align(matA, matB)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"procrustes: alignment failed",
			err,
		))
	}

	nSamples, nDim := matA.Dims()
	_ = nSamples

	rotationFlat := rotationToFlat(result.R, nDim)
	procrustes.artifact.Poke(result.Residual, "output", "value")
	procrustes.artifact.Poke(rotationFlat, "output", "rotation")
	procrustes.artifact.Poke(float64(nDim), "output", "nDim")
	state.MergeOutput("value", result.Residual)
	state.MergeOutput("rotation", rotationFlat)
	state.MergeOutput("nDim", float64(nDim))
	state.Poke("output", "root")
	state.Poke([]string{"value", "rotation", "nDim"}, "inputs")

	return state.Read(payload)
}

func (procrustes *Procrustes) Write(payload []byte) (int, error) {
	procrustes.artifact.WithPayload(payload)
	return len(payload), nil
}

func (procrustes *Procrustes) Close() error {
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

	uMat, _, vMat, err := procrustes.jacobiSVD(&cross)

	if err != nil {
		return ProcrustesResult{}, err
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

func matricesFromArtifact(artifact *datura.Artifact) (*mat.Dense, *mat.Dense, error) {
	nSamples := int(datura.Peek[float64](artifact, "nSamples"))
	nDim := int(datura.Peek[float64](artifact, "nDim"))

	nestedA := datura.Peek[[][]float64](artifact, "rowsA")
	nestedB := datura.Peek[[][]float64](artifact, "rowsB")

	if len(nestedA) > 0 && len(nestedB) > 0 {
		if nSamples <= 0 {
			nSamples = len(nestedA)
		}

		if nDim <= 0 && len(nestedA) > 0 {
			nDim = len(nestedA[0])
		}

		matA, err := denseFromRows(nestedA, nSamples, nDim)

		if err != nil {
			return nil, nil, err
		}

		matB, err := denseFromRows(nestedB, nSamples, nDim)

		if err != nil {
			return nil, nil, err
		}

		return matA, matB, nil
	}

	flatA := datura.Peek[[]float64](artifact, "rowsA")
	flatB := datura.Peek[[]float64](artifact, "rowsB")

	if nSamples <= 0 || nDim <= 0 || len(flatA) != nSamples*nDim || len(flatB) != nSamples*nDim {
		return nil, nil, procrustesError("matrix attributes invalid")
	}

	return mat.NewDense(nSamples, nDim, flatA), mat.NewDense(nSamples, nDim, flatB), nil
}

func rowsToFlat(rows [][]float64, nSamples, nDim int) []float64 {
	flat := make([]float64, nSamples*nDim)

	for rowIdx := range nSamples {
		copy(flat[rowIdx*nDim:(rowIdx+1)*nDim], rows[rowIdx])
	}

	return flat
}

func rotationToFlat(rotation *mat.Dense, nDim int) []float64 {
	flat := make([]float64, nDim*nDim)

	for rowIdx := 0; rowIdx < nDim; rowIdx++ {
		for colIdx := 0; colIdx < nDim; colIdx++ {
			flat[rowIdx*nDim+colIdx] = rotation.At(rowIdx, colIdx)
		}
	}

	return flat
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

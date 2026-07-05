package geometry

import (
	"math/rand"
	"testing"

	"gonum.org/v1/gonum/mat"
)

func TestProcrustesAlignIdentity(t *testing.T) {
	nDim := 8
	nSamples := 12
	matA := randomMatrix(nSamples, nDim, 42)
	matB := copyMatrix(matA)
	denseA, err := denseFromRows(matA, nSamples, nDim)
	if err != nil {
		t.Fatal(err)
	}

	denseB, err := denseFromRows(matB, nSamples, nDim)
	if err != nil {
		t.Fatal(err)
	}

	stage := &Procrustes{}
	result, err := stage.align(denseA, denseB)
	if err != nil {
		t.Fatal(err)
	}

	if result.Residual > 1e-10 {
		t.Fatalf("residual = %e, want near zero", result.Residual)
	}

	for row := 0; row < nDim; row++ {
		for col := 0; col < nDim; col++ {
			expected := 0.0

			if row == col {
				expected = 1
			}

			if delta := result.R.At(row, col) - expected; delta > 1e-8 || delta < -1e-8 {
				t.Fatalf("rotation[%d,%d] = %f, want %f", row, col, result.R.At(row, col), expected)
			}
		}
	}
}

func TestProcrustesDecompose(t *testing.T) {
	stage := &Procrustes{}
	dense := mat.NewDense(3, 3, []float64{
		3, 0, 0,
		0, 5, 0,
		0, 0, 2,
	})

	_, sigma, _, err := stage.Decompose(dense)
	if err != nil {
		t.Fatal(err)
	}

	if len(sigma) != 3 {
		t.Fatalf("sigma len = %d, want 3", len(sigma))
	}
}

func BenchmarkProcrustesAlign(testingTB *testing.B) {
	nDim := 32
	nSamples := 16
	matA := randomMatrix(nSamples, nDim, 1337)
	matB := copyMatrix(matA)
	denseA, err := denseFromRows(matA, nSamples, nDim)
	if err != nil {
		testingTB.Fatal(err)
	}

	denseB, err := denseFromRows(matB, nSamples, nDim)
	if err != nil {
		testingTB.Fatal(err)
	}

	stage := &Procrustes{}
	testingTB.ReportAllocs()

	for testingTB.Loop() {
		if _, err := stage.align(denseA, denseB); err != nil {
			testingTB.Fatal(err)
		}
	}
}

func randomMatrix(rows, cols int, seed int64) [][]float64 {
	rng := rand.New(rand.NewSource(seed))
	output := make([][]float64, rows)

	for row := range rows {
		output[row] = make([]float64, cols)

		for col := range cols {
			output[row][col] = rng.NormFloat64()
		}
	}

	return output
}

func copyMatrix(input [][]float64) [][]float64 {
	output := make([][]float64, len(input))

	for row := range input {
		output[row] = append([]float64(nil), input[row]...)
	}

	return output
}

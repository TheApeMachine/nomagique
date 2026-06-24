package geometry

import (
	"math"
	"math/rand"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"gonum.org/v1/gonum/mat"
)

func TestProcrustes_Observe(t *testing.T) {
	Convey("Given the Procrustes orthogonal alignment solver", t, func() {
		Convey("When A equals B (identity alignment)", func() {
			nDim := 8
			nSamples := 12

			matA := randomMatrix(nSamples, nDim, 42)
			matB := copyMatrix(matA)

			stage, err := NewProcrustesFromRows(
				datura.Acquire("procrustes-config", datura.APPJSON),
				matA, matB, nSamples, nDim,
			)
			So(err, ShouldBeNil)

			artifact := datura.Acquire("test", datura.APPJSON)
			err = transport.NewFlipFlop(artifact, stage)

			So(err, ShouldBeNil)

			residual := datura.Peek[float64](artifact, "output", "value")
			rotationFlat := datura.Peek[[]float64](artifact, "output", "rotation")

			Convey("It should return R ≈ I with near-zero residual", func() {
				So(residual, ShouldBeLessThan, 1e-10)

				for row := 0; row < nDim; row++ {
					for col := 0; col < nDim; col++ {
						expected := 0.0

						if row == col {
							expected = 1.0
						}

						So(rotationFlat[row*nDim+col], ShouldAlmostEqual, expected, 1e-8)
					}
				}
			})
		})

		Convey("When B = R·A for a known 90° rotation in the first two dims", func() {
			nDim := 4
			nSamples := 6

			matA := randomMatrix(nSamples, nDim, 99)
			knownR := knownRotation90(nDim)
			matB := applyRotation(knownR, matA, nSamples, nDim)

			stage, err := NewProcrustesFromRows(
				datura.Acquire("procrustes-config", datura.APPJSON),
				matA, matB, nSamples, nDim,
			)
			So(err, ShouldBeNil)

			artifact := datura.Acquire("test", datura.APPJSON)
			err = transport.NewFlipFlop(artifact, stage)

			So(err, ShouldBeNil)

			Convey("It should recover the known rotation with low residual", func() {
				residual := datura.Peek[float64](artifact, "output", "value")
				rotationFlat := datura.Peek[[]float64](artifact, "output", "rotation")
				So(residual, ShouldBeLessThan, 1e-8)

				for row := 0; row < nDim; row++ {
					for col := 0; col < nDim; col++ {
						So(rotationFlat[row*nDim+col], ShouldAlmostEqual, knownR.At(row, col), 1e-6)
					}
				}
			})
		})

		Convey("When given degenerate inputs", func() {
			Convey("It should error on dimension mismatch", func() {
				config := datura.Acquire("procrustes-config", datura.APPJSON).
					Poke(float64(1), "nSamples").
					Poke(float64(2), "nDim").
					Poke([]float64{1, 2}, "rowsA").
					Poke([]float64{1, 2, 3, 4}, "rowsB")
				stage := NewProcrustes(config)
				artifact := datura.Acquire("test", datura.APPJSON)
				err := transport.NewFlipFlop(artifact, stage)

				So(err, ShouldNotBeNil)
			})

			Convey("It should error on row count mismatch", func() {
				matA := randomMatrix(3, 2, 1)
				matB := randomMatrix(4, 2, 2)
				config := datura.Acquire("procrustes-config", datura.APPJSON).
					Poke(float64(3), "nSamples").
					Poke(float64(2), "nDim").
					Poke(rowsToFlat(matA, 3, 2), "rowsA").
					Poke(rowsToFlat(matB, 4, 2), "rowsB")
				stage := NewProcrustes(config)
				artifact := datura.Acquire("test", datura.APPJSON)
				err := transport.NewFlipFlop(artifact, stage)

				So(err, ShouldNotBeNil)
			})
		})
	})
}

func TestProcrustes_Decompose(t *testing.T) {
	Convey("Given the Jacobi SVD decomposition", t, func() {
		stage := NewProcrustes(datura.Acquire("procrustes-config", datura.APPJSON))

		Convey("When decomposing a known diagonal matrix", func() {
			dense := mat.NewDense(3, 3, []float64{
				3, 0, 0,
				0, 5, 0,
				0, 0, 2,
			})

			uMat, sigma, vMat, err := stage.Decompose(dense)
			So(err, ShouldBeNil)

			Convey("It should recover the singular values", func() {
				So(len(sigma), ShouldEqual, 3)

				for _, value := range sigma {
					So(singularValuesMatch(value, []float64{5, 3, 2}, 1e-8), ShouldBeTrue)
				}

				So(uMat, ShouldNotBeNil)
				So(vMat, ShouldNotBeNil)
			})
		})

		Convey("When decomposing a general matrix", func() {
			dense := mat.NewDense(3, 2, []float64{
				1, 2,
				3, 4,
				5, 6,
			})

			uMat, sigma, vMat, err := stage.Decompose(dense)
			So(err, ShouldBeNil)

			Convey("It should reconstruct A = U·Σ·Vᵀ", func() {
				reconstructed := reconstructFromSVD(uMat, sigma, vMat, 3, 2)

				for row := 0; row < 3; row++ {
					for col := 0; col < 2; col++ {
						So(reconstructed[row][col], ShouldAlmostEqual, dense.At(row, col), 1e-8)
					}
				}
			})
		})

		Convey("When rows < cols", func() {
			Convey("It should return an error", func() {
				dense := mat.NewDense(1, 3, []float64{1, 2, 3})
				_, _, _, err := stage.Decompose(dense)

				So(err, ShouldNotBeNil)
			})
		})
	})
}

func BenchmarkProcrustes_Observe(b *testing.B) {
	nDim := 512
	nSamples := 16

	matA := randomMatrix(nSamples, nDim, 1337)
	matB := randomMatrix(nSamples, nDim, 7331)
	stage, err := NewProcrustesFromRows(
		datura.Acquire("procrustes-config-bench", datura.APPJSON),
		matA, matB, nSamples, nDim,
	)

	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()

	for b.Loop() {
		artifact := datura.Acquire("test", datura.APPJSON)
		_ = transport.NewFlipFlop(artifact, stage)
	}
}

func BenchmarkProcrustes_Decompose(b *testing.B) {
	nDim := 512
	matRows := randomMatrix(nDim, nDim, 2024)
	dense, err := denseFromRows(matRows, nDim, nDim)

	if err != nil {
		b.Fatal(err)
	}

	stage := NewProcrustes(datura.Acquire("procrustes-config-bench", datura.APPJSON))

	b.ReportAllocs()

	for b.Loop() {
		_, _, _, _ = stage.Decompose(dense)
	}
}

func randomMatrix(rows, cols int, seed int64) [][]float64 {
	rng := rand.New(rand.NewSource(seed))
	matOut := make([][]float64, rows)

	for row := range matOut {
		matOut[row] = make([]float64, cols)

		for col := range matOut[row] {
			matOut[row][col] = rng.NormFloat64()
		}
	}

	return matOut
}

func copyMatrix(src [][]float64) [][]float64 {
	dst := make([][]float64, len(src))

	for row := range src {
		dst[row] = make([]float64, len(src[row]))
		copy(dst[row], src[row])
	}

	return dst
}

func eye(nDim int) *mat.Dense {
	identity := mat.NewDense(nDim, nDim, nil)

	for rowIdx := 0; rowIdx < nDim; rowIdx++ {
		identity.Set(rowIdx, rowIdx, 1)
	}

	return identity
}

func knownRotation90(nDim int) *mat.Dense {
	rotation := eye(nDim)
	rotation.Set(0, 0, 0)
	rotation.Set(0, 1, -1)
	rotation.Set(1, 0, 1)
	rotation.Set(1, 1, 0)

	return rotation
}

func applyRotation(rotation *mat.Dense, matA [][]float64, nSamples, nDim int) [][]float64 {
	out := make([][]float64, nSamples)

	for sampleIdx := 0; sampleIdx < nSamples; sampleIdx++ {
		out[sampleIdx] = make([]float64, nDim)

		for dimIdx := 0; dimIdx < nDim; dimIdx++ {
			var sum float64

			for innerIdx := 0; innerIdx < nDim; innerIdx++ {
				sum += rotation.At(dimIdx, innerIdx) * matA[sampleIdx][innerIdx]
			}

			out[sampleIdx][dimIdx] = sum
		}
	}

	return out
}

func reconstructFromSVD(
	uMat *mat.Dense, sigma []float64, vMat *mat.Dense, rows, cols int,
) [][]float64 {
	out := make([][]float64, rows)

	for rowIdx := 0; rowIdx < rows; rowIdx++ {
		out[rowIdx] = make([]float64, cols)

		for colIdx := 0; colIdx < cols; colIdx++ {
			var sum float64

			for innerIdx := 0; innerIdx < cols; innerIdx++ {
				sum += uMat.At(rowIdx, innerIdx) * sigma[innerIdx] * vMat.At(colIdx, innerIdx)
			}

			out[rowIdx][colIdx] = sum
		}
	}

	return out
}

func singularValuesMatch(value float64, expected []float64, tolerance float64) bool {
	for _, target := range expected {
		if math.Abs(value-target) < tolerance {
			return true
		}
	}

	return false
}

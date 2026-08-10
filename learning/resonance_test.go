package learning

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"gonum.org/v1/gonum/mat"
)

func TestNewResonanceManifold(testingTB *testing.T) {
	Convey("Given a valid architecture and learning pace", testingTB, func() {
		manifold := NewResonanceManifold([]int{4, 8, 4}, 2, 0.01)

		Convey("It should construct vector state and matrix operators", func() {
			So(manifold, ShouldNotBeNil)
			So(manifold.z, ShouldHaveLength, 3)
			So(manifold.z[0].Len(), ShouldEqual, 4)
			So(manifold.z[1].Len(), ShouldEqual, 8)
			So(manifold.z[2].Len(), ShouldEqual, 4)
			So(manifold.V, ShouldNotBeNil)
		})
	})

	Convey("Given an invalid architecture or learning pace", testingTB, func() {
		Convey("It should reject construction", func() {
			So(NewResonanceManifold([]int{4}, 1, 0.01), ShouldBeNil)
			So(NewResonanceManifold([]int{4, 2}, 1, 0), ShouldBeNil)
		})
	})
}

func TestResonanceManifoldSettle(testingTB *testing.T) {
	Convey("Given a manifold settling a sequence of inputs", testingTB, func() {
		manifold := NewResonanceManifold([]int{4, 8, 4}, 0, 0.03)
		inputs := [][]float64{
			{0.8, -0.2, 0.4, 0.1},
			{-0.3, 0.6, -0.1, 0.2},
			{0.1, 0.2, -0.4, 0.7},
		}

		for _, input := range inputs {
			So(manifold.Settle(input, true), ShouldBeNil)
		}

		Convey("It should keep every reported quantity finite", func() {
			So(finite(manifold.Energy()), ShouldBeTrue)
			So(finite(manifold.PredictionEnergy()), ShouldBeTrue)
			So(finite(manifold.ReconstructionError()), ShouldBeTrue)

			for _, latent := range manifold.z {
				for _, value := range latent.RawVector().Data {
					So(finite(value), ShouldBeTrue)
				}
			}
		})

		Convey("It should reuse its inference workspace", func() {
			var settleErr error
			allocations := testing.AllocsPerRun(100, func() {
				settleErr = manifold.Settle(inputs[0], true)
			})

			So(settleErr, ShouldBeNil)
			So(allocations, ShouldEqual, 0)
		})
	})
}

func TestResonanceManifoldWireSnapshot(testingTB *testing.T) {
	Convey("Given a settled manifold with active latent regularization", testingTB, func() {
		manifold := NewResonanceManifold([]int{4, 8, 4}, 0, 0.03)
		So(manifold.Settle([]float64{0.8, -0.2, 0.4, 0.1}, true), ShouldBeNil)
		manifold.cfg.LatentDecay = 100
		manifold.cfg.Sparsity = 100

		_, surprise, energy := manifold.WireSnapshot()

		Convey("It should report per-residual prediction diagnostics", func() {
			predictionDimensions := float64(manifold.arch[0] + manifold.arch[1])
			So(surprise, ShouldAlmostEqual,
				manifold.ReconstructionError()/math.Sqrt(float64(manifold.arch[0])))
			So(energy, ShouldAlmostEqual,
				manifold.PredictionEnergy()/predictionDimensions)
			So(energy, ShouldBeLessThan, manifold.Energy())
		})
	})
}

func TestResonanceManifoldStateGradients(testingTB *testing.T) {
	Convey("Given an unregularized manifold with fixed latent state", testingTB, func() {
		manifold := NewResonanceManifold([]int{2, 3, 2}, 0, 0.03)
		manifold.cfg.UsePrecision = false
		manifold.cfg.LatentDecay = 0
		manifold.cfg.Sparsity = 0
		manifold.cfg.GradClip = math.Inf(1)
		manifold.temporalPriorReady = false
		manifold.z[0].CopyVec(mat.NewVecDense(2, []float64{0.3, -0.4}))
		manifold.z[1].CopyVec(mat.NewVecDense(3, []float64{0.2, -0.1, 0.5}))
		manifold.z[2].CopyVec(mat.NewVecDense(2, []float64{-0.3, 0.6}))

		predictions, layerErrors := manifold.predictAdjacentLayers()
		gradients := manifold.stateGradients(predictions, layerErrors)
		gradientCopies := make([][]float64, len(gradients))

		for layerIndex := 1; layerIndex < len(gradients); layerIndex++ {
			gradientCopies[layerIndex] = append(
				[]float64(nil),
				gradients[layerIndex].RawVector().Data...,
			)
		}

		Convey("It should match the central difference of prediction energy", func() {
			finiteDifferenceStep := math.Cbrt(math.Nextafter(1, 2) - 1)

			for layerIndex := 1; layerIndex < len(manifold.z); layerIndex++ {
				latent := manifold.z[layerIndex]

				for valueIndex, analytical := range gradientCopies[layerIndex] {
					original := latent.AtVec(valueIndex)
					latent.SetVec(valueIndex, original+finiteDifferenceStep)
					positiveEnergy := manifold.PredictionEnergy()
					latent.SetVec(valueIndex, original-finiteDifferenceStep)
					negativeEnergy := manifold.PredictionEnergy()
					latent.SetVec(valueIndex, original)

					numerical := (positiveEnergy - negativeEnergy) /
						(2 * finiteDifferenceStep)
					So(analytical, ShouldAlmostEqual, numerical, 1e-8)
				}
			}
		})
	})
}

func TestResonanceManifoldProjectTemporalOperatorNorm(testingTB *testing.T) {
	Convey("Given a temporal operator above its contraction limit", testingTB, func() {
		manifold := NewResonanceManifold([]int{2, 2}, 0, 0.05)
		manifold.A.Copy(mat.NewDense(2, 2, []float64{10, 4, -3, 8}))
		valuesBuffer := &manifold.workspace.svdValues[0]

		err := manifold.projectTemporalOperatorNorm()

		var decomposition mat.SVD
		So(decomposition.Factorize(manifold.A, mat.SVDNone), ShouldBeTrue)
		operatorNorm := decomposition.Values(make([]float64, 2))[0]

		Convey("It should reuse the workspace and restore contraction", func() {
			So(err, ShouldBeNil)
			So(&manifold.workspace.svdValues[0], ShouldEqual, valuesBuffer)
			So(operatorNorm, ShouldBeLessThanOrEqualTo,
				manifold.cfg.TemporalNormMax+1e-12)
		})
	})
}

func BenchmarkResonanceManifoldSettle(testingTB *testing.B) {
	manifold := NewResonanceManifold([]int{8, 16, 8}, 0, 0.01)
	input := []float64{0.1, -0.2, 0.3, -0.4, 0.5, -0.6, 0.7, -0.8}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		if err := manifold.Settle(input, true); err != nil {
			testingTB.Fatal(err)
		}
	}
}

func BenchmarkResonanceManifoldWireSnapshot(testingTB *testing.B) {
	manifold := NewResonanceManifold([]int{8, 16, 8}, 0, 0.01)
	input := []float64{0.1, -0.2, 0.3, -0.4, 0.5, -0.6, 0.7, -0.8}

	if err := manifold.Settle(input, true); err != nil {
		testingTB.Fatal(err)
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		manifold.WireSnapshot()
	}
}

func BenchmarkResonanceManifoldLearn(testingTB *testing.B) {
	manifold := NewResonanceManifold([]int{8, 16, 8}, 1, 0.01)
	input := []float64{0.1, -0.2, 0.3, -0.4, 0.5, -0.6, 0.7, -0.8}
	target := []float64{0.01}

	if err := manifold.Settle(input, false); err != nil {
		testingTB.Fatal(err)
	}

	if err := manifold.Learn(target); err != nil {
		testingTB.Fatal(err)
	}

	if err := manifold.Settle(input, false); err != nil {
		testingTB.Fatal(err)
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		if err := manifold.Learn(target); err != nil {
			testingTB.Fatal(err)
		}
	}
}

func BenchmarkResonanceManifoldProjectTemporalOperatorNorm(testingTB *testing.B) {
	manifold := NewResonanceManifold([]int{8, 8}, 0, 0.01)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		if err := manifold.projectTemporalOperatorNorm(); err != nil {
			testingTB.Fatal(err)
		}
	}
}

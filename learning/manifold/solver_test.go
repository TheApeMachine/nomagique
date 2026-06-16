//go:build darwin && cgo

package manifold

import (
	"math"
	"testing"

	"github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/learning"
)

func maxAbsDiff(a, b []float64) float64 {
	worst := 0.0
	for index := range a {
		diff := math.Abs(a[index] - b[index])
		if diff > worst {
			worst = diff
		}
	}
	return worst
}

func TestNewResonanceSolver(t *testing.T) {
	convey.Convey("Given a valid architecture", t, func() {
		solver, err := NewSolver([]int{4, 8, 4}, 2, 0.01)

		convey.Convey("It should construct a usable solver", func() {
			convey.So(err, convey.ShouldBeNil)
			convey.So(solver, convey.ShouldNotBeNil)

			defer solver.Close()

			convey.So(solver.Settle([]float64{0.8, -0.2, 0.4, 0.1}, true), convey.ShouldBeNil)

			latent, latentErr := solver.LatentState()
			convey.So(latentErr, convey.ShouldBeNil)
			convey.So(len(latent), convey.ShouldEqual, 4)

			for _, value := range latent {
				convey.So(math.IsNaN(value), convey.ShouldBeFalse)
				convey.So(math.IsInf(value, 0), convey.ShouldBeFalse)
			}
		})
	})
}

func TestResonanceSolver_InitialWeightsMatchReference(t *testing.T) {
	convey.Convey("Given the seeded weight initialization", t, func() {
		arch := []int{4, 8, 4}
		solver, err := NewSolver(arch, 2, 0.01)
		convey.So(err, convey.ShouldBeNil)
		defer solver.Close()

		reference, refErr := learning.NewResonanceManifold(arch, 2, 0.01)
		convey.So(refErr, convey.ShouldBeNil)

		w, _, _, _, weightsErr := solver.Weights()
		convey.So(weightsErr, convey.ShouldBeNil)

		convey.Convey("The GPU W[0] should equal the gonum reference W[0]", func() {
			rows, cols := reference.W[0].Dims()
			convey.So(rows*cols, convey.ShouldEqual, len(w[:rows*cols]))

			worst := 0.0
			for r := 0; r < rows; r++ {
				for c := 0; c < cols; c++ {
					diff := math.Abs(float64(w[r*cols+c]) - reference.W[0].At(r, c))
					if diff > worst {
						worst = diff
					}
				}
			}
			convey.So(worst, convey.ShouldBeLessThan, 1e-6)
		})
	})
}

func TestResonanceSolver_SettleParity(t *testing.T) {
	convey.Convey("Given identical fresh GPU and gonum manifolds", t, func() {
		arch := []int{4, 8, 4}
		alpha := 0.05

		solver, err := NewSolver(arch, 0, alpha)
		convey.So(err, convey.ShouldBeNil)
		defer solver.Close()

		reference, refErr := learning.NewResonanceManifold(arch, 0, alpha)
		convey.So(refErr, convey.ShouldBeNil)

		input := []float64{0.8, -0.2, 0.4, 0.1}

		convey.So(solver.Settle(input, false), convey.ShouldBeNil)
		convey.So(reference.Settle(input, false), convey.ShouldBeNil)

		gpuLatent, latentErr := solver.LatentState()
		convey.So(latentErr, convey.ShouldBeNil)
		refLatent := reference.LatentState()

		gpuEnergy, energyErr := solver.Energy()
		convey.So(energyErr, convey.ShouldBeNil)
		refEnergy := reference.Energy()

		convey.Convey("Latent state and energy should match within float32 tolerance", func() {
			convey.So(maxAbsDiff(gpuLatent, refLatent), convey.ShouldBeLessThan, 1e-3)
			convey.So(math.Abs(gpuEnergy-refEnergy), convey.ShouldBeLessThan, 1e-3)
		})
	})
}

func TestResonanceSolver_SettleLearnParity(t *testing.T) {
	convey.Convey("Given a settle+learn cycle on both backends", t, func() {
		arch := []int{4, 8, 4}
		alpha := 0.05

		solver, err := NewSolver(arch, 2, alpha)
		convey.So(err, convey.ShouldBeNil)
		defer solver.Close()

		reference, refErr := learning.NewResonanceManifold(arch, 2, alpha)
		convey.So(refErr, convey.ShouldBeNil)

		input := []float64{0.8, -0.2, 0.4, 0.1}
		target := []float64{0.5, -0.5}

		for i := 0; i < 5; i++ {
			convey.So(solver.Settle(input, true), convey.ShouldBeNil)
			convey.So(solver.Learn(target), convey.ShouldBeNil)
			reference.SettleFromBatchOptions(input, target, true, true)
		}

		gpuLatent, latentErr := solver.LatentState()
		convey.So(latentErr, convey.ShouldBeNil)
		refLatent := reference.LatentState()

		convey.Convey("Latent trajectories should stay close after learning", func() {
			for _, value := range gpuLatent {
				convey.So(math.IsNaN(value), convey.ShouldBeFalse)
			}
			convey.So(maxAbsDiff(gpuLatent, refLatent), convey.ShouldBeLessThan, 5e-2)
		})
	})
}

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
	for i := range a {
		d := math.Abs(a[i] - b[i])
		if d > worst {
			worst = d
		}
	}
	return worst
}

// distinct deterministic input per slot.
func slotInput(slot, dim int) []float64 {
	in := make([]float64, dim)
	for j := 0; j < dim; j++ {
		in[j] = math.Sin(float64(slot*7+j) * 0.31)
	}
	return in
}

func slotTarget(slot, dim int) []float64 {
	t := make([]float64, dim)
	for j := 0; j < dim; j++ {
		t[j] = math.Cos(float64(slot*3+j) * 0.5)
	}
	return t
}

func TestBatchSolver_Construct(t *testing.T) {
	convey.Convey("Given a valid batched architecture", t, func() {
		s, err := NewBatchSolver([]int{4, 8, 4}, 2, 16, 0.01)
		convey.So(err, convey.ShouldBeNil)
		convey.So(s, convey.ShouldNotBeNil)
		defer s.Close()
		convey.So(s.Batch(), convey.ShouldEqual, 16)
	})
}

// The core test: N symbols settled in lockstep must each match an independent
// gonum manifold fed that symbol's own input.
func TestBatchSolver_SettleParityPerSlot(t *testing.T) {
	convey.Convey("Given a batch of distinct inputs", t, func() {
		arch := []int{4, 8, 4}
		alpha := 0.05
		N := 32

		s, err := NewBatchSolver(arch, 0, N, alpha)
		convey.So(err, convey.ShouldBeNil)
		defer s.Close()

		refs := make([]*learning.ResonanceManifold, N)
		for i := 0; i < N; i++ {
			refs[i], _ = learning.NewResonanceManifold(arch, 0, alpha)
		}

		for slot := 0; slot < N; slot++ {
			convey.So(s.SetInput(slot, slotInput(slot, arch[0]), nil), convey.ShouldBeNil)
		}
		convey.So(s.Settle(false), convey.ShouldBeNil)
		for slot := 0; slot < N; slot++ {
			convey.So(refs[slot].Settle(slotInput(slot, arch[0]), false), convey.ShouldBeNil)
		}

		convey.Convey("Each slot's latent matches its gonum twin within float32 tol", func() {
			worst := 0.0
			for slot := 0; slot < N; slot++ {
				gpu, _ := s.LatentState(slot)
				ref := refs[slot].LatentState()
				d := maxAbsDiff(gpu, ref)
				if d > worst {
					worst = d
				}
			}
			convey.So(worst, convey.ShouldBeLessThan, 2e-3)
		})
	})
}

// Per-symbol weights diverge correctly: each slot learns its own stream and must
// track its independent gonum twin across multiple settle+learn cycles.
func TestBatchSolver_SettleLearnParityPerSlot(t *testing.T) {
	convey.Convey("Given per-slot settle+learn cycles", t, func() {
		arch := []int{4, 8, 4}
		alpha := 0.05
		N := 16

		s, err := NewBatchSolver(arch, 2, N, alpha)
		convey.So(err, convey.ShouldBeNil)
		defer s.Close()

		refs := make([]*learning.ResonanceManifold, N)
		for i := 0; i < N; i++ {
			refs[i], _ = learning.NewResonanceManifold(arch, 2, alpha)
		}

		for cycle := 0; cycle < 5; cycle++ {
			for slot := 0; slot < N; slot++ {
				convey.So(s.SetInput(slot, slotInput(slot, arch[0]), slotTarget(slot, 2)), convey.ShouldBeNil)
			}
			convey.So(s.Settle(true), convey.ShouldBeNil)
			convey.So(s.Learn(), convey.ShouldBeNil)
			for slot := 0; slot < N; slot++ {
				_, _ = refs[slot].SettleFromBatchOptions(slotInput(slot, arch[0]), slotTarget(slot, 2), true, true)
			}
		}

		convey.Convey("Each slot tracks its independent gonum twin after learning", func() {
			worst := 0.0
			for slot := 0; slot < N; slot++ {
				gpu, _ := s.LatentState(slot)
				ref := refs[slot].LatentState()
				d := maxAbsDiff(gpu, ref)
				if d > worst {
					worst = d
				}
			}
			convey.So(worst, convey.ShouldBeLessThan, 5e-2)
		})
	})
}

func TestBatchSolver_ReadOutcomesParity(t *testing.T) {
	convey.Convey("Given a settled and learned batch", t, func() {
		arch := []int{4, 8, 4}
		alpha := 0.05
		batchSize := 8

		solver, err := NewBatchSolver(arch, 0, batchSize, alpha)
		convey.So(err, convey.ShouldBeNil)
		defer solver.Close()

		references := make([]*learning.ResonanceManifold, batchSize)

		for slot := 0; slot < batchSize; slot++ {
			references[slot], _ = learning.NewResonanceManifold(arch, 0, alpha)
			convey.So(solver.SetInput(slot, slotInput(slot, arch[0]), nil), convey.ShouldBeNil)
		}

		convey.So(solver.Settle(true), convey.ShouldBeNil)
		convey.So(solver.Learn(), convey.ShouldBeNil)
		convey.So(solver.ReadOutcomes(), convey.ShouldBeNil)

		for slot := 0; slot < batchSize; slot++ {
			_, _ = references[slot].SettleFromBatchOptions(slotInput(slot, arch[0]), nil, true, true)
		}

		convey.Convey("Batch outcomes should match per-slot reads and gonum twins", func() {
			latentWorst := 0.0
			energyWorst := 0.0
			surpriseWorst := 0.0

			for slot := 0; slot < batchSize; slot++ {
				batchLatent, batchEnergy, batchSurprise, outcomeErr := solver.OutcomeSlot(slot)
				convey.So(outcomeErr, convey.ShouldBeNil)

				slotLatent, latentErr := solver.LatentState(slot)
				convey.So(latentErr, convey.ShouldBeNil)

				slotEnergy, energyErr := solver.Energy(slot)
				convey.So(energyErr, convey.ShouldBeNil)

				slotSurprise, surpriseErr := solver.ReconstructionError(slot)
				convey.So(surpriseErr, convey.ShouldBeNil)

				latentWorst = math.Max(latentWorst, maxAbsDiff(batchLatent, slotLatent))
				energyWorst = math.Max(energyWorst, math.Abs(batchEnergy-slotEnergy))
				surpriseWorst = math.Max(surpriseWorst, math.Abs(batchSurprise-slotSurprise))

				referenceLatent := references[slot].LatentState()
				latentWorst = math.Max(latentWorst, maxAbsDiff(batchLatent, referenceLatent))
				energyWorst = math.Max(energyWorst, math.Abs(batchEnergy-references[slot].Energy()))
				surpriseWorst = math.Max(surpriseWorst, math.Abs(batchSurprise-references[slot].ReconstructionError()))
			}

			convey.So(latentWorst, convey.ShouldBeLessThan, 5e-2)
			convey.So(energyWorst, convey.ShouldBeLessThan, 5e-1)
			convey.So(surpriseWorst, convey.ShouldBeLessThan, 5e-1)
		})
	})
}

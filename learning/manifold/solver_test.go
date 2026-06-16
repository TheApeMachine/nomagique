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
				refs[slot].SettleFromBatchOptions(slotInput(slot, arch[0]), slotTarget(slot, 2), true, true)
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

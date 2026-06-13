package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique"
)

func TestShift_Observe(testingTB *testing.T) {
	Convey("Given matching reference and live distributions", testingTB, func() {
		reference := nomagique.Numbers(1, 1, 1, 1)
		live := nomagique.Numbers(1, 1, 1, 1)
		shift := NewShift(reference, live, nil, 0, 0)
		divergence := shift.Observe()

		Convey("It should return zero drift", func() {
			So(float64(divergence), ShouldAlmostEqual, 0, 1e-9)
		})
	})

	Convey("Given diverging reference and live distributions", testingTB, func() {
		reference := nomagique.Numbers(4, 1, 1, 1)
		live := nomagique.Numbers(1, 1, 1, 4)
		shift := NewShift(reference, live, nil, 0, 0)
		divergence := shift.Observe()

		Convey("It should return positive drift", func() {
			So(float64(divergence), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkShift_Observe(testingTB *testing.B) {
	reference := nomagique.Numbers(1, 2, 3, 4)
	live := nomagique.Numbers(1, 1, 2, 4)
	shift := NewShift(reference, live, nil, 0, 0)

	for testingTB.Loop() {
		_ = shift.Observe()
	}
}

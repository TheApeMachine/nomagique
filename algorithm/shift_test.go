package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestShift_Observe(testingTB *testing.T) {
	Convey("Given matching reference and live distributions", testingTB, func() {
		shift := NewShift[float64](
			[]float64{1, 1, 1, 1},
			[]float64{1, 1, 1, 1},
			nil, 0, 0,
		)
		divergence := shift.Observe()

		Convey("It should return zero drift", func() {
			So(float64(divergence), ShouldAlmostEqual, 0, 1e-9)
		})
	})

	Convey("Given diverging reference and live distributions", testingTB, func() {
		shift := NewShift[float64](
			[]float64{4, 1, 1, 1},
			[]float64{1, 1, 1, 4},
			nil, 0, 0,
		)
		divergence := shift.Observe()

		Convey("It should return positive drift", func() {
			So(float64(divergence), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkShift_Observe(testingTB *testing.B) {
	shift := NewShift[float64](
		[]float64{1, 2, 3, 4},
		[]float64{1, 1, 2, 4},
		nil, 0, 0,
	)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = shift.Observe()
	}
}

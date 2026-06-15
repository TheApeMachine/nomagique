package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestIntervalCoupling_Observe(testingTB *testing.T) {
	Convey("Given proportional interval histories", testingTB, func() {
		left := NewIntervalSeries[float64](8)
		right := NewIntervalSeries[float64](8)

		observeEpochLevel(left, 1_000, 100)
		observeEpochLevel(left, 2_000, 110)
		observeEpochLevel(right, 1_000, 50)
		observeEpochLevel(right, 2_000, 55)

		coupling := NewIntervalCoupling[float64](left, right)
		value := coupling.Observe()

		Convey("It should estimate unit correlation", func() {
			So(float64(value), ShouldAlmostEqual, 1, 1e-9)
		})
	})
}

func BenchmarkIntervalCoupling_Observe(testingTB *testing.B) {
	left := NewIntervalSeries[float64](64)
	right := NewIntervalSeries[float64](64)

	for step := range 64 {
		observeEpochLevel(left, int64((step+1)*1_000), 100+float64(step)*0.1)
		observeEpochLevel(right, int64((step+1)*1_000), 50+float64(step)*0.05)
	}

	coupling := NewIntervalCoupling[float64](left, right)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = coupling.Observe()
	}
}

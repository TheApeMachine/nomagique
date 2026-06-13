package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestIntervalCoupling_Observe(testingTB *testing.T) {
	Convey("Given proportional interval histories", testingTB, func() {
		left := NewIntervalSeries(8)
		right := NewIntervalSeries(8)

		left.Observe(1_000, 100)
		left.Observe(2_000, 110)
		right.Observe(1_000, 50)
		right.Observe(2_000, 55)

		coupling := NewIntervalCoupling(left, right)
		value := coupling.Observe()

		Convey("It should estimate unit correlation", func() {
			So(float64(value), ShouldAlmostEqual, 1, 1e-9)
		})
	})
}

func BenchmarkIntervalCoupling_Observe(testingTB *testing.B) {
	left := NewIntervalSeries(64)
	right := NewIntervalSeries(64)

	for step := range 64 {
		nanos := int64((step + 1) * 1_000)
		left.Observe(nanos, 100+float64(step)*0.1)
		right.Observe(nanos, 50+float64(step)*0.05)
	}

	coupling := NewIntervalCoupling(left, right)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = coupling.Observe()
	}
}

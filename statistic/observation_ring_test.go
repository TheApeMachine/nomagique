package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestObservationRingObserve(testingTB *testing.T) {
	Convey("Given an observation ring", testingTB, func() {
		ring := NewObservationRing()

		for _, value := range []float64{0.7, 0.8, 0.9, 0.95} {
			ring.Observe(value)
		}

		Convey("It should derive quantiles from retained samples", func() {
			So(ring.Quantile(0.75), ShouldAlmostEqual, 0.9, 1e-9)
			So(ring.Median(), ShouldAlmostEqual, 0.85, 1e-9)
		})
	})
}

func BenchmarkObservationRingObserve(b *testing.B) {
	ring := NewObservationRing()

	for b.Loop() {
		ring.Observe(0.1)
	}
}

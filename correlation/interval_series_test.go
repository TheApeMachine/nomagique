package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestIntervalSeriesObserve(testingTB *testing.T) {
	Convey("Given an interval series", testingTB, func() {
		series := NewIntervalSeries(8)

		Convey("It should accumulate log-return intervals", func() {
			series.Observe(1_000, 100)
			series.Observe(2_000, 110)

			So(series.Len(), ShouldEqual, 1)
			So(series.LastReturnMagnitude(), ShouldBeGreaterThan, 0)
		})

		Convey("It should correlate proportional interval streams", func() {
			left := NewIntervalSeries(8)
			right := NewIntervalSeries(8)

			left.Observe(1_000, 100)
			left.Observe(2_000, 110)
			right.Observe(1_000, 50)
			right.Observe(2_000, 55)

			correlation, ok := IntervalCorrelation(left, right)

			So(ok, ShouldBeTrue)
			So(correlation, ShouldAlmostEqual, 1, 1e-9)
		})
	})
}

func BenchmarkIntervalCorrelation(testingTB *testing.B) {
	left := NewIntervalSeries(128)
	right := NewIntervalSeries(128)

	for index := range 128 {
		nanos := int64((index + 1) * 1_000)
		left.Observe(nanos, 100+float64(index)*0.1)
		right.Observe(nanos, 50+float64(index)*0.05)
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_, _ = IntervalCorrelation(left, right)
	}
}

package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func observeEpochLevel[T ~float64](
	stage interface {
		Observe(...core.Number[T]) core.Scalar[T]
	},
	epoch int64,
	level float64,
) {
	stage.Observe(
		core.Scalar[T](float64(epoch)),
		core.Scalar[T](level),
	)
}

func TestIntervalSeriesObserve(testingTB *testing.T) {
	Convey("Given an interval series", testingTB, func() {
		series := NewIntervalSeries[float64](8)

		Convey("It should accumulate log-return intervals", func() {
			observeEpochLevel(series, 1_000, 100)
			observeEpochLevel(series, 2_000, 110)

			So(series.Len(), ShouldEqual, 1)
			So(series.LastReturnMagnitude(), ShouldBeGreaterThan, 0)
		})

		Convey("It should correlate proportional interval streams", func() {
			left := NewIntervalSeries[float64](8)
			right := NewIntervalSeries[float64](8)

			observeEpochLevel(left, 1_000, 100)
			observeEpochLevel(left, 2_000, 110)
			observeEpochLevel(right, 1_000, 50)
			observeEpochLevel(right, 2_000, 55)

			correlation, ok := IntervalCorrelation(left, right)

			So(ok, ShouldBeTrue)
			So(correlation, ShouldAlmostEqual, 1, 1e-9)
		})
	})
}

func BenchmarkIntervalCorrelation(testingTB *testing.B) {
	left := NewIntervalSeries[float64](128)
	right := NewIntervalSeries[float64](128)

	for index := range 128 {
		epoch := int64((index + 1) * 1_000)
		observeEpochLevel(left, epoch, 100+float64(index)*0.1)
		observeEpochLevel(right, epoch, 50+float64(index)*0.05)
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_, _ = IntervalCorrelation(left, right)
	}
}

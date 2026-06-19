package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestIntervalSeriesObserve(testingTB *testing.T) {
	Convey("Given an interval series", testingTB, func() {
		series := NewIntervalSeries(8)

		Convey("It should accumulate log-return intervals", func() {
			artifact := datura.Acquire("test", datura.APPJSON).
				Poke(float64(1_000), "sample").
				Poke(100.0, "paired")
			err := transport.NewFlipFlop(artifact, series)

			So(err, ShouldBeNil)

			artifact.Poke(float64(2_000), "sample").Poke(110.0, "paired")
			err = transport.NewFlipFlop(artifact, series)

			So(err, ShouldBeNil)
			So(series.Len(), ShouldEqual, 1)
			So(series.LastReturnMagnitude(), ShouldBeGreaterThan, 0)
		})

		Convey("It should correlate proportional interval streams", func() {
			left := NewIntervalSeries(8)
			right := NewIntervalSeries(8)
			artifact := datura.Acquire("test", datura.APPJSON)

			artifact.Poke(float64(1_000), "sample").Poke(100.0, "paired")
			err := transport.NewFlipFlop(artifact, left)

			So(err, ShouldBeNil)

			artifact.Poke(float64(2_000), "sample").Poke(110.0, "paired")
			err = transport.NewFlipFlop(artifact, left)

			So(err, ShouldBeNil)

			artifact.Poke(float64(1_000), "sample").Poke(50.0, "paired")
			err = transport.NewFlipFlop(artifact, right)

			So(err, ShouldBeNil)

			artifact.Poke(float64(2_000), "sample").Poke(55.0, "paired")
			err = transport.NewFlipFlop(artifact, right)

			So(err, ShouldBeNil)

			correlation, ok := IntervalCorrelation(left, right)

			So(ok, ShouldBeTrue)
			So(correlation, ShouldAlmostEqual, 1, 1e-9)
		})
	})
}

func BenchmarkIntervalCorrelation(testingTB *testing.B) {
	left := NewIntervalSeries(128)
	right := NewIntervalSeries(128)
	artifact := datura.Acquire("test", datura.APPJSON)

	for index := range 128 {
		epoch := float64((index + 1) * 1_000)
		artifact.Poke(epoch, "sample").Poke(100+float64(index)*0.1, "paired")
		_ = transport.NewFlipFlop(artifact, left)
		artifact.Poke(epoch, "sample").Poke(50+float64(index)*0.05, "paired")
		_ = transport.NewFlipFlop(artifact, right)
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_, _ = IntervalCorrelation(left, right)
	}
}

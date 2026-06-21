package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestIntervalSeriesObserve(testingTB *testing.T) {
	Convey("Given an interval series", testingTB, func() {
		series := NewIntervalSeries(datura.Acquire("interval-series-config", datura.APPJSON))

		Convey("It should accumulate log-return intervals", func() {
			artifact := datura.Acquire("test", datura.APPJSON).
				Poke(float64(1_000), "sample").
				Poke(100.0, "paired")
			err := transport.NewFlipFlop(artifact, series)

			So(err, ShouldBeNil)

			artifact.Poke(float64(2_000), "sample").Poke(110.0, "paired")
			err = transport.NewFlipFlop(artifact, series)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldBeGreaterThan, 0)
		})

		Convey("It should correlate proportional interval streams", func() {
			coupling := NewIntervalCoupling(datura.Acquire("interval-coupling-config", datura.APPJSON))
			artifact := datura.Acquire("test", datura.APPJSON)

			artifact.Poke(0, "config", "side").
				Poke(float64(1_000), "sample").
				Poke(100.0, "paired")
			err := transport.NewFlipFlop(artifact, coupling)

			So(err, ShouldBeNil)

			artifact.Poke(0, "config", "side").
				Poke(float64(2_000), "sample").
				Poke(110.0, "paired")
			err = transport.NewFlipFlop(artifact, coupling)

			So(err, ShouldBeNil)

			artifact.Poke(1, "config", "side").
				Poke(float64(1_000), "sample").
				Poke(50.0, "paired")
			err = transport.NewFlipFlop(artifact, coupling)

			So(err, ShouldBeNil)

			artifact.Poke(1, "config", "side").
				Poke(float64(2_000), "sample").
				Poke(55.0, "paired")
			err = transport.NewFlipFlop(artifact, coupling)

			So(err, ShouldBeNil)

			value := datura.Peek[float64](artifact, "output", "value")

			So(value, ShouldAlmostEqual, 1, 1e-9)
		})
	})
}

func BenchmarkIntervalCorrelation(testingTB *testing.B) {
	coupling := NewIntervalCoupling(datura.Acquire("interval-coupling-config", datura.APPJSON))
	artifact := datura.Acquire("test", datura.APPJSON)

	for index := range 128 {
		epoch := float64((index + 1) * 1_000)
		artifact.Poke(0, "config", "side").Poke(epoch, "sample").Poke(100+float64(index)*0.1, "paired")
		_ = transport.NewFlipFlop(artifact, coupling)
		artifact.Poke(1, "config", "side").Poke(epoch, "sample").Poke(50+float64(index)*0.05, "paired")
		_ = transport.NewFlipFlop(artifact, coupling)
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = transport.NewFlipFlop(artifact, coupling)
	}
}

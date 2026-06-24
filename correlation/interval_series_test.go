package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestIntervalSeriesObserve(testingTB *testing.T) {
	Convey("Given an interval series", testingTB, func() {
		series := NewIntervalSeries(IntervalWireConfig("interval-series-config"))

		Convey("It should accumulate log-return intervals", func() {
			artifact := EpochLevelWire(datura.Acquire("test", datura.APPJSON), float64(1_000), 100.0)
			err := transport.NewFlipFlop(artifact, series)

			So(err, ShouldNotBeNil)

			artifact = EpochLevelWire(datura.Acquire("test", datura.APPJSON), float64(2_000), 110.0)
			err = transport.NewFlipFlop(artifact, series)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldBeGreaterThan, 0)
		})

		Convey("It should correlate proportional interval streams", func() {
			coupling := NewIntervalCoupling(IntervalWireConfig("interval-coupling-config"))
			artifact := datura.Acquire("test", datura.APPJSON)

			artifact.Poke(0, "config", "side")
			artifact = EpochLevelWire(artifact, float64(1_000), 100.0)
			err := transport.NewFlipFlop(artifact, coupling)

			So(err, ShouldNotBeNil)

			artifact.Poke(0, "config", "side")
			artifact = EpochLevelWire(artifact, float64(2_000), 110.0)
			err = transport.NewFlipFlop(artifact, coupling)

			So(err, ShouldNotBeNil)

			artifact.Poke(1, "config", "side")
			artifact = EpochLevelWire(artifact, float64(1_000), 50.0)
			err = transport.NewFlipFlop(artifact, coupling)

			So(err, ShouldNotBeNil)

			artifact.Poke(1, "config", "side")
			artifact = EpochLevelWire(artifact, float64(2_000), 55.0)
			err = transport.NewFlipFlop(artifact, coupling)

			So(err, ShouldBeNil)

			value := datura.Peek[float64](artifact, "output", "value")

			So(value, ShouldAlmostEqual, 1, 1e-9)
		})
	})
}

func BenchmarkIntervalCorrelation(testingTB *testing.B) {
	coupling := NewIntervalCoupling(IntervalWireConfig("interval-coupling-config"))
	artifact := datura.Acquire("test", datura.APPJSON)

	for index := range 128 {
		epoch := float64((index + 1) * 1_000)
		artifact.Poke(0, "config", "side")
		artifact = EpochLevelWire(artifact, epoch, 100+float64(index)*0.1)
		_ = transport.NewFlipFlop(artifact, coupling)
		artifact.Poke(1, "config", "side")
		artifact = EpochLevelWire(artifact, epoch, 50+float64(index)*0.05)
		_ = transport.NewFlipFlop(artifact, coupling)
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = transport.NewFlipFlop(artifact, coupling)
	}
}

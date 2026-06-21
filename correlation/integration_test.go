package correlation_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/correlation"
)

func TestIntegration(t *testing.T) {
	Convey("Given correlation stages composed through nomagique.Number", t, func() {
		Convey("When Pearson receives a perfectly correlated batch", func() {
			artifact := datura.Acquire("test", datura.APPJSON).
				Poke([]float64{1, 2, 1, 2}, "batch")
			pipeline := nomagique.Number(
				correlation.NewPearson(datura.Acquire("pearson-config", datura.APPJSON)),
			)
			err := transport.NewFlipFlop(artifact, pipeline)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 1)
		})

		Convey("When IntervalSeries streams epoch and level pairs", func() {
			artifact := datura.Acquire("test", datura.APPJSON)
			series := nomagique.Number(
				correlation.NewIntervalSeries(datura.Acquire("interval-series-config", datura.APPJSON)),
			)

			artifact.Poke(float64(1_000), "sample").Poke(100.0, "paired")
			err := transport.NewFlipFlop(artifact, series)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 0)

			artifact.Poke(float64(2_000), "sample").Poke(110.0, "paired")
			err = transport.NewFlipFlop(artifact, series)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldBeGreaterThan, 0)
		})

		Convey("When Contagion runs on correlated members", func() {
			contagion := nomagique.Number(
				correlation.NewContagion(
					datura.Acquire("test", datura.APPJSON).
						Poke(2, "config", "minSamples").
						Poke(2, "config", "memberCap").
						Poke(2, "config", "adaptiveSigma").
						Poke(4, "config", "tier", "fast").
						Poke(8, "config", "tier", "medium").
						Poke(16, "config", "tier", "slow"),
				),
			)
			artifact := datura.Acquire("test", datura.APPJSON)

			for step := range 16 {
				epoch := float64((step + 1) * 1_000)
				artifact.Poke(1, "member").Poke(epoch, "sample").Poke(100+float64(step)*0.1, "paired")
				err := transport.NewFlipFlop(artifact, contagion)

				So(err, ShouldBeNil)

				artifact.Poke(2, "member").Poke(epoch, "sample").Poke(50+float64(step)*0.05, "paired")
				err = transport.NewFlipFlop(artifact, contagion)

				So(err, ShouldBeNil)
			}

			So(datura.Peek[float64](artifact, "output", "value"), ShouldBeGreaterThan, 0)
		})
	})
}

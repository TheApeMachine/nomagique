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
			pipeline := nomagique.Number(correlation.NewPearson(nil))
			err := transport.NewFlipFlop(artifact, pipeline)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 1)
		})

		Convey("When IntervalSeries streams epoch and level pairs", func() {
			artifact := datura.Acquire("test", datura.APPJSON)
			series := nomagique.Number(correlation.NewIntervalSeries(8))

			artifact.Poke(float64(1_000), "sample").Poke(100.0, "paired")
			err := transport.NewFlipFlop(artifact, series)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 0)

			artifact.Poke(float64(2_000), "sample").Poke(110.0, "paired")
			err = transport.NewFlipFlop(artifact, series)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldBeGreaterThan, 0)
		})

		Convey("When WindowSet and Contagion run on correlated members", func() {
			first := correlation.NewWindowSet(16)
			second := correlation.NewWindowSet(16)
			artifact := datura.Acquire("test", datura.APPJSON)

			for step := range 16 {
				epoch := float64((step + 1) * 1_000)
				artifact.Poke(epoch, "sample").Poke(100+float64(step)*0.1, "paired")
				err := transport.NewFlipFlop(artifact, first)

				So(err, ShouldBeNil)

				artifact.Poke(epoch, "sample").Poke(50+float64(step)*0.05, "paired")
				err = transport.NewFlipFlop(artifact, second)

				So(err, ShouldBeNil)
			}

			contagion := nomagique.Number(correlation.NewContagion(
				[]*correlation.WindowSet{first, second},
				correlation.TierWindows{Fast: 4, Medium: 8, Slow: 16},
				correlation.ContagionConfig{MinSamples: 2, MemberCap: 2, AdaptiveSigma: 2},
			))

			trigger := datura.Acquire("test", datura.APPJSON)
			err := transport.NewFlipFlop(trigger, contagion)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](trigger, "output", "value"), ShouldBeGreaterThan, 0)
		})
	})
}

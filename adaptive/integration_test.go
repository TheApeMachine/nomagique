package adaptive_test

import (
	"math"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/adaptive"
)

func TestIntegration(t *testing.T) {
	Convey("Given adaptive primitives composed through nomagique.Number", t, func() {
		Convey("When EMA stages volatility before Delta on a trending series", func() {
			artifact := datura.Acquire("test", datura.APPJSON).Poke(10, "sample")

			pipeline := nomagique.Number(adaptive.NewEMA(nil), adaptive.NewDelta())
			err := transport.NewFlipFlop(artifact, pipeline)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 0)

			Convey("It should bootstrap then emit unit-normalized deltas", func() {
				artifact.Poke(20, "sample")
				err := transport.NewFlipFlop(artifact, pipeline)

				So(err, ShouldBeNil)

				delta := datura.Peek[float64](artifact, "output", "value")

				So(delta, ShouldBeGreaterThanOrEqualTo, 0)
				So(delta, ShouldBeLessThanOrEqualTo, 1)
			})

			Convey("It should match a freshly composed reference pipeline", func() {
				reference := datura.Acquire("test", datura.APPJSON).Poke(10, "sample")
				referencePipeline := nomagique.Number(adaptive.NewEMA(nil), adaptive.NewDelta())
				err := transport.NewFlipFlop(reference, referencePipeline)

				So(err, ShouldBeNil)
				So(
					datura.Peek[float64](reference, "output", "value"),
					ShouldEqual,
					datura.Peek[float64](artifact, "output", "value"),
				)
			})

			Convey("It should retain EMA state across sequential FlipFlops", func() {
				retained := datura.Acquire("test", datura.APPJSON).Poke(10, "sample")
				exponential := adaptive.NewEMA(nil)

				err := transport.NewFlipFlop(retained, exponential)

				So(err, ShouldBeNil)
				So(datura.Peek[float64](retained, "output", "value"), ShouldEqual, 10)

				retained.Poke(20, "sample")
				err = transport.NewFlipFlop(retained, exponential)

				So(err, ShouldBeNil)
				So(datura.Peek[float64](retained, "output", "value"), ShouldEqual, 20)
			})
		})

		Convey("When EMA, Variance, and ZScore run in sequence", func() {
			artifact := datura.Acquire("test", datura.APPJSON).Poke(10, "sample")

			pipeline := nomagique.Number(
				adaptive.NewEMA(nil),
				adaptive.NewVariance(),
				adaptive.NewZScore(),
			)
			err := transport.NewFlipFlop(artifact, pipeline)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 0)

			Convey("It should warm up then emit finite surprise scores", func() {
				artifact.Poke(22, "sample")
				err := transport.NewFlipFlop(artifact, pipeline)

				So(err, ShouldBeNil)

				surprise := datura.Peek[float64](artifact, "output", "value")

				So(math.IsNaN(surprise), ShouldBeFalse)
				So(math.IsInf(surprise, 0), ShouldBeFalse)
				So(surprise, ShouldBeGreaterThanOrEqualTo, 0)
			})

			Convey("It should keep variance on a parallel EMA-Variance path", func() {
				varianceArtifact := datura.Acquire("test", datura.APPJSON).Poke(22, "sample")
				variancePipeline := nomagique.Number(adaptive.NewEMA(nil), adaptive.NewVariance())
				err := transport.NewFlipFlop(varianceArtifact, variancePipeline)

				So(err, ShouldBeNil)
				So(datura.Peek[float64](varianceArtifact, "output", "value"), ShouldBeGreaterThanOrEqualTo, 0)
			})
		})

		Convey("When Range and Momentum normalize a volatile series", func() {
			artifact := datura.Acquire("test", datura.APPJSON).Poke(1, "sample")

			pipeline := nomagique.Number(adaptive.NewRange(), adaptive.NewMomentum())
			err := transport.NewFlipFlop(artifact, pipeline)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 0)

			Convey("It should bootstrap then emit signed unit-normalized momentum", func() {
				artifact.Poke(3, "sample")
				err := transport.NewFlipFlop(artifact, pipeline)

				So(err, ShouldBeNil)

				momentum := datura.Peek[float64](artifact, "output", "value")

				So(momentum, ShouldBeGreaterThanOrEqualTo, -1)
				So(momentum, ShouldBeLessThanOrEqualTo, 1)
				So(math.IsNaN(momentum), ShouldBeFalse)
				So(math.IsInf(momentum, 0), ShouldBeFalse)
			})

			Convey("It should accept a second FlipFlop on the same pipeline", func() {
				artifact.Poke(3, "sample")
				err := transport.NewFlipFlop(artifact, pipeline)

				So(err, ShouldBeNil)
			})
		})

		Convey("When TimeElastic follows Range on timed observations", func() {
			artifact := datura.Acquire("test", datura.APPJSON).
				Poke(10, "sample").
				Poke(float64(time.Unix(0, int64(time.Hour)).UnixNano()), "at")

			pipeline := nomagique.Number(
				adaptive.NewRange(),
				adaptive.NewTimeElastic(time.Hour, 1e-6),
			)
			err := transport.NewFlipFlop(artifact, pipeline)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 1)

			Convey("It should bootstrap at unity then stay finite and positive", func() {
				artifact.Poke(14, "sample").
					Poke(float64(time.Unix(0, int64(5*time.Hour)).UnixNano()), "at")
				err := transport.NewFlipFlop(artifact, pipeline)

				So(err, ShouldBeNil)

				relative := datura.Peek[float64](artifact, "output", "value")

				So(relative, ShouldBeGreaterThan, 0)
				So(math.IsNaN(relative), ShouldBeFalse)
				So(math.IsInf(relative, 0), ShouldBeFalse)
			})
		})
	})
}

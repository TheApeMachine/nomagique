package adaptive_test

import (
	"io"
	"math"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/adaptive"
)

func emaConfigArtifact(name string) *datura.Artifact {
	return datura.Acquire(name, datura.APPJSON).
		Poke(2, "period").
		Poke(2, "smoothing")
}

func TestIntegration(t *testing.T) {
	Convey("Given adaptive primitives composed through nomagique.Number", t, func() {
		Convey("When EMA stages volatility before Delta on a trending series", func() {
			artifact := datura.Acquire("test", datura.APPJSON).Poke(10, "sample")
			emaConfig := emaConfigArtifact("ema-config")
			deltaConfig := datura.Acquire("delta-config", datura.APPJSON)

			pipeline := nomagique.Number(adaptive.NewEMA(emaConfig), adaptive.NewDelta(deltaConfig))
			err := transport.NewFlipFlop(artifact, pipeline)

			So(err, ShouldNotBeNil)

			Convey("It should bootstrap then emit unit-normalized deltas", func() {
				artifact.Poke(20, "sample")
				err := transport.NewFlipFlop(artifact, pipeline)

				So(err, ShouldNotBeNil)

				artifact.Poke(30, "sample")
				err = transport.NewFlipFlop(artifact, pipeline)

				So(err, ShouldBeIn, nil, io.EOF)

				delta := datura.Peek[float64](artifact, "output", "value")

				So(delta, ShouldBeGreaterThanOrEqualTo, 0)
				So(delta, ShouldBeLessThanOrEqualTo, 1)
			})

			Convey("It should match a freshly composed reference pipeline", func() {
				mainArtifact := datura.Acquire("test", datura.APPJSON).Poke(10, "sample")
				referenceArtifact := datura.Acquire("test", datura.APPJSON).Poke(10, "sample")
				mainPipeline := nomagique.Number(
					adaptive.NewEMA(emaConfigArtifact("ema-main-config")),
					adaptive.NewDelta(datura.Acquire("delta-main-config", datura.APPJSON)),
				)
				referencePipeline := nomagique.Number(
					adaptive.NewEMA(emaConfigArtifact("ema-ref-config")),
					adaptive.NewDelta(datura.Acquire("delta-ref-config", datura.APPJSON)),
				)

				err := transport.NewFlipFlop(mainArtifact, mainPipeline)

				So(err, ShouldNotBeNil)

				err = transport.NewFlipFlop(referenceArtifact, referencePipeline)

				So(err, ShouldNotBeNil)

				mainArtifact.Poke(20, "sample")
				referenceArtifact.Poke(20, "sample")
				err = transport.NewFlipFlop(mainArtifact, mainPipeline)

				So(err, ShouldNotBeNil)

				err = transport.NewFlipFlop(referenceArtifact, referencePipeline)

				So(err, ShouldNotBeNil)

				mainArtifact.Poke(30, "sample")
				referenceArtifact.Poke(30, "sample")
				err = transport.NewFlipFlop(mainArtifact, mainPipeline)

				So(err, ShouldBeIn, nil, io.EOF)

				err = transport.NewFlipFlop(referenceArtifact, referencePipeline)

				So(err, ShouldBeIn, nil, io.EOF)
				So(
					datura.Peek[float64](referenceArtifact, "output", "value"),
					ShouldEqual,
					datura.Peek[float64](mainArtifact, "output", "value"),
				)
			})

			Convey("It should retain EMA state across sequential FlipFlops", func() {
				retained := datura.Acquire("test", datura.APPJSON).Poke(10, "sample")
				exponential := adaptive.NewEMA(emaConfigArtifact("ema-config"))

				err := transport.NewFlipFlop(retained, exponential)

				So(err, ShouldNotBeNil)

				retained.Poke(20, "sample")
				err = transport.NewFlipFlop(retained, exponential)

				So(err, ShouldBeNil)
				So(datura.Peek[float64](retained, "output", "value"), ShouldBeGreaterThan, 0)
			})
		})

		Convey("When EMA, Variance, and ZScore run in sequence", func() {
			artifact := datura.Acquire("test", datura.APPJSON).Poke(10, "sample")
			emaConfig := emaConfigArtifact("ema-config")
			varianceConfig := datura.Acquire("variance-config", datura.APPJSON)
			zscoreConfig := datura.Acquire("zscore-config", datura.APPJSON)

			pipeline := nomagique.Number(
				adaptive.NewEMA(emaConfig),
				adaptive.NewVariance(varianceConfig),
				adaptive.NewZScore(zscoreConfig),
			)
			err := transport.NewFlipFlop(artifact, pipeline)

			So(err, ShouldNotBeNil)

			Convey("It should warm up then emit finite surprise scores", func() {
				artifact.Poke(22, "sample")
				err := transport.NewFlipFlop(artifact, pipeline)

				So(err, ShouldNotBeNil)

				artifact.Poke(30, "sample")
				err = transport.NewFlipFlop(artifact, pipeline)

				So(err, ShouldNotBeNil)

				artifact.Poke(40, "sample")
				err = transport.NewFlipFlop(artifact, pipeline)

				So(err, ShouldBeNil)

				surprise := datura.Peek[float64](artifact, "output", "value")

				So(math.IsNaN(surprise), ShouldBeFalse)
				So(math.IsInf(surprise, 0), ShouldBeFalse)
				So(surprise, ShouldNotEqual, 0)
			})

			Convey("It should keep variance on a parallel EMA-Variance path", func() {
				parallelEMA := emaConfigArtifact("ema-parallel-config")
				parallelVariance := datura.Acquire("variance-parallel-config", datura.APPJSON)
				varianceArtifact := datura.Acquire("test", datura.APPJSON).Poke(10, "sample")
				variancePipeline := nomagique.Number(
					adaptive.NewEMA(parallelEMA),
					adaptive.NewVariance(parallelVariance),
				)
				err := transport.NewFlipFlop(varianceArtifact, variancePipeline)

				So(err, ShouldNotBeNil)

				varianceArtifact.Poke(22, "sample")
				err = transport.NewFlipFlop(varianceArtifact, variancePipeline)

				So(err, ShouldNotBeNil)

				varianceArtifact.Poke(30, "sample")
				err = transport.NewFlipFlop(varianceArtifact, variancePipeline)

				So(err, ShouldBeNil)
				So(datura.Peek[float64](varianceArtifact, "output", "value"), ShouldBeGreaterThan, 0)
			})
		})

		Convey("When Range and Momentum normalize a volatile series", func() {
			artifact := datura.Acquire("test", datura.APPJSON).Poke(1, "sample")
			rangeConfig := datura.Acquire("range-config", datura.APPJSON)
			momentumConfig := datura.Acquire("momentum-config", datura.APPJSON)

			pipeline := nomagique.Number(adaptive.NewRange(rangeConfig), adaptive.NewMomentum(momentumConfig))
			err := transport.NewFlipFlop(artifact, pipeline)

			So(err, ShouldNotBeNil)

			Convey("It should bootstrap then emit signed unit-normalized momentum", func() {
				artifact.Poke(3, "sample")
				So(transport.NewFlipFlop(artifact, pipeline), ShouldNotBeNil)

				artifact.Poke(5, "sample")
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
				So(transport.NewFlipFlop(artifact, pipeline), ShouldNotBeNil)

				artifact.Poke(5, "sample")
				err := transport.NewFlipFlop(artifact, pipeline)

				So(err, ShouldBeNil)
			})
		})

		Convey("When TimeElastic follows Range on timed observations", func() {
			artifact := datura.Acquire("test", datura.APPJSON).
				Poke(10, "sample").
				Poke(float64(time.Unix(0, int64(time.Hour)).UnixNano()), "at")
			rangeConfig := datura.Acquire("range-config", datura.APPJSON)
			timeElasticConfig := datura.Acquire("time-elastic-config", datura.APPJSON).
				Poke(float64(time.Hour), "config", "halflife").
				Poke(1e-6, "config", "epsilon")

			pipeline := nomagique.Number(
				adaptive.NewRange(rangeConfig),
				adaptive.NewTimeElastic(timeElasticConfig),
			)
			err := transport.NewFlipFlop(artifact, pipeline)

			So(err, ShouldNotBeNil)

			Convey("It should bootstrap at unity then stay finite and positive", func() {
				second := datura.Acquire("test", datura.APPJSON).
					Poke(14, "sample").
					Poke(float64(time.Unix(0, int64(5*time.Hour)).UnixNano()), "at")
				err := transport.NewFlipFlop(second, pipeline)

				So(err, ShouldBeIn, nil, io.EOF)
				So(datura.Peek[float64](second, "output", "value"), ShouldEqual, 1)

				relative := datura.Peek[float64](second, "output", "value")

				So(relative, ShouldBeGreaterThan, 0)
				So(math.IsNaN(relative), ShouldBeFalse)
				So(math.IsInf(relative, 0), ShouldBeFalse)
			})
		})
	})
}

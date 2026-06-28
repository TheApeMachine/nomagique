package adaptive_test

import (
	"io"
	"math"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/adaptive"
)

func emaConfigArtifact(name string) *datura.Artifact {
	return datura.Acquire(name, datura.APPJSON).
		Poke("sample", "input").
		Poke(2, "period").
		Poke(2, "smoothing")
}

func TestIntegration(t *testing.T) {
	Convey("Given adaptive primitives composed through nomagique.Number", t, func() {
		Convey("When EMA stages volatility before Delta on a trending series", func() {
			artifact := adaptive.ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10)
			emaConfig := emaConfigArtifact("ema-config")
			deltaConfig := datura.Acquire("delta-config", datura.APPJSON).Poke("value", "input")

			pipeline := nomagique.Number(adaptive.NewEMA(emaConfig), adaptive.NewDelta(deltaConfig))
			err := nomagique.RoundTripArtifact(artifact, pipeline)

			So(err, ShouldBeIn, nil, io.EOF)

			Convey("It should bootstrap then emit unit-normalized deltas", func() {
				adaptive.ScalarWire(artifact, "sample", 20)
				err := nomagique.RoundTripArtifact(artifact, pipeline)

				So(err, ShouldBeIn, nil, io.EOF)

				adaptive.ScalarWire(artifact, "sample", 30)
				err = nomagique.RoundTripArtifact(artifact, pipeline)

				So(err, ShouldBeIn, nil, io.EOF)

				delta := datura.Peek[float64](artifact, "output", "value")

				So(delta, ShouldBeGreaterThanOrEqualTo, 0)
				So(delta, ShouldBeLessThanOrEqualTo, 1)
			})

			Convey("It should match a freshly composed reference pipeline", func() {
				mainArtifact := adaptive.ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10)
				referenceArtifact := adaptive.ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10)
				mainPipeline := nomagique.Number(
					adaptive.NewEMA(emaConfigArtifact("ema-main-config")),
					adaptive.NewDelta(datura.Acquire("delta-main-config", datura.APPJSON).Poke("value", "input")),
				)
				referencePipeline := nomagique.Number(
					adaptive.NewEMA(emaConfigArtifact("ema-ref-config")),
					adaptive.NewDelta(datura.Acquire("delta-ref-config", datura.APPJSON).Poke("value", "input")),
				)

				err := nomagique.RoundTripArtifact(mainArtifact, mainPipeline)

				So(err, ShouldBeIn, nil, io.EOF)

				err = nomagique.RoundTripArtifact(referenceArtifact, referencePipeline)

				So(err, ShouldBeIn, nil, io.EOF)

				adaptive.ScalarWire(mainArtifact, "sample", 20)
				adaptive.ScalarWire(referenceArtifact, "sample", 20)
				err = nomagique.RoundTripArtifact(mainArtifact, mainPipeline)

				So(err, ShouldBeIn, nil, io.EOF)

				err = nomagique.RoundTripArtifact(referenceArtifact, referencePipeline)

				So(err, ShouldBeIn, nil, io.EOF)

				adaptive.ScalarWire(mainArtifact, "sample", 30)
				adaptive.ScalarWire(referenceArtifact, "sample", 30)
				err = nomagique.RoundTripArtifact(mainArtifact, mainPipeline)

				So(err, ShouldBeIn, nil, io.EOF)

				err = nomagique.RoundTripArtifact(referenceArtifact, referencePipeline)

				So(err, ShouldBeIn, nil, io.EOF)
				So(
					datura.Peek[float64](referenceArtifact, "output", "value"),
					ShouldEqual,
					datura.Peek[float64](mainArtifact, "output", "value"),
				)
			})

			Convey("It should retain EMA state across sequential FlipFlops", func() {
				retained := adaptive.ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10)
				exponential := adaptive.NewEMA(emaConfigArtifact("ema-config"))

				err := nomagique.RoundTripArtifact(retained, exponential)

				So(err, ShouldBeIn, nil, io.EOF)

				adaptive.ScalarWire(retained, "sample", 20)
				err = nomagique.RoundTripArtifact(retained, exponential)

				So(err, ShouldBeNil)
				So(datura.Peek[float64](retained, "output", "value"), ShouldBeGreaterThan, 0)
			})
		})

		Convey("When EMA, Variance, and ZScore run in sequence", func() {
			artifact := adaptive.ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10)
			emaConfig := emaConfigArtifact("ema-config")
			varianceConfig := datura.Acquire("variance-config", datura.APPJSON).Poke("value", "input")
			zscoreConfig := datura.Acquire("zscore-config", datura.APPJSON).Poke("value", "input")

			pipeline := nomagique.Number(
				adaptive.NewEMA(emaConfig),
				adaptive.NewVariance(varianceConfig),
				adaptive.NewZScore(zscoreConfig),
			)
			err := nomagique.RoundTripArtifact(artifact, pipeline)

			So(err, ShouldBeIn, nil, io.EOF)
			So(datura.Peek[bool](artifact, "output", "ready"), ShouldBeFalse)

			Convey("It should warm up then emit finite surprise scores", func() {
				adaptive.ScalarWire(artifact, "sample", 22)
				err := nomagique.RoundTripArtifact(artifact, pipeline)

				So(err, ShouldBeIn, nil, io.EOF)

				adaptive.ScalarWire(artifact, "sample", 30)
				err = nomagique.RoundTripArtifact(artifact, pipeline)

				So(err, ShouldBeIn, nil, io.EOF)

				adaptive.ScalarWire(artifact, "sample", 40)
				err = nomagique.RoundTripArtifact(artifact, pipeline)

				So(err, ShouldBeNil)

				surprise := datura.Peek[float64](artifact, "output", "value")

				So(math.IsNaN(surprise), ShouldBeFalse)
				So(math.IsInf(surprise, 0), ShouldBeFalse)
				So(surprise, ShouldNotEqual, 0)
			})

			Convey("It should keep variance on a parallel EMA-Variance path", func() {
				parallelEMA := emaConfigArtifact("ema-parallel-config")
				parallelVariance := datura.Acquire("variance-parallel-config", datura.APPJSON).Poke("value", "input")
				varianceArtifact := adaptive.ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10)
				variancePipeline := nomagique.Number(
					adaptive.NewEMA(parallelEMA),
					adaptive.NewVariance(parallelVariance),
				)
				err := nomagique.RoundTripArtifact(varianceArtifact, variancePipeline)

				So(err, ShouldBeIn, nil, io.EOF)
				So(datura.Peek[bool](varianceArtifact, "output", "ready"), ShouldBeFalse)

				adaptive.ScalarWire(varianceArtifact, "sample", 22)
				err = nomagique.RoundTripArtifact(varianceArtifact, variancePipeline)

				So(err, ShouldBeIn, nil, io.EOF)

				adaptive.ScalarWire(varianceArtifact, "sample", 30)
				err = nomagique.RoundTripArtifact(varianceArtifact, variancePipeline)

				So(err, ShouldBeNil)
				So(datura.Peek[float64](varianceArtifact, "output", "value"), ShouldBeGreaterThan, 0)
			})
		})

		Convey("When Range and Momentum normalize a volatile series", func() {
			rangeConfig := datura.Acquire("range-config", datura.APPJSON).Poke("sample", "input")
			momentumConfig := datura.Acquire("momentum-config", datura.APPJSON).Poke("value", "input")
			pipeline := nomagique.Number(adaptive.NewRange(rangeConfig), adaptive.NewMomentum(momentumConfig))

			artifact := adaptive.ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 1)
			err := nomagique.RoundTripArtifact(artifact, pipeline)

			So(err, ShouldBeIn, nil, io.EOF)
			So(datura.Peek[bool](artifact, "output", "ready"), ShouldBeFalse)

			Convey("It should bootstrap then emit signed unit-normalized momentum", func() {
				artifact = adaptive.ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 3)
				So(nomagique.RoundTripArtifact(artifact, pipeline), ShouldBeIn, nil, io.EOF)

				artifact = adaptive.ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 5)
				err := nomagique.RoundTripArtifact(artifact, pipeline)

				So(err, ShouldBeNil)

				momentum := datura.Peek[float64](artifact, "output", "value")

				So(momentum, ShouldBeGreaterThanOrEqualTo, -1)
				So(momentum, ShouldBeLessThanOrEqualTo, 1)
				So(math.IsNaN(momentum), ShouldBeFalse)
				So(math.IsInf(momentum, 0), ShouldBeFalse)
			})

			Convey("It should accept a second FlipFlop on the same pipeline", func() {
				artifact = adaptive.ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 3)
				So(nomagique.RoundTripArtifact(artifact, pipeline), ShouldBeIn, nil, io.EOF)

				artifact = adaptive.ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 5)
				err := nomagique.RoundTripArtifact(artifact, pipeline)

				So(err, ShouldBeNil)
			})
		})

		Convey("When TimeElastic follows Range on timed observations", func() {
			artifact := adaptive.ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10)
			artifact.Merge("at", float64(time.Unix(0, int64(time.Hour)).UnixNano()))
			rangeConfig := datura.Acquire("range-config", datura.APPJSON).Poke("sample", "input")
			timeElasticConfig := datura.Acquire("time-elastic-config", datura.APPJSON).
				Poke("value", "input").
				Poke(float64(time.Hour), "config", "halflife").
				Poke(1e-6, "config", "epsilon")

			pipeline := nomagique.Number(
				adaptive.NewRange(rangeConfig),
				adaptive.NewTimeElastic(timeElasticConfig),
			)
			err := nomagique.RoundTripArtifact(artifact, pipeline)

			So(err, ShouldBeIn, nil, io.EOF)
			So(datura.Peek[bool](artifact, "output", "ready"), ShouldBeFalse)

			Convey("It should bootstrap at unity then stay finite and positive", func() {
				second := adaptive.ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 14)
				second.Merge("at", float64(time.Unix(0, int64(5*time.Hour)).UnixNano()))
				err := nomagique.RoundTripArtifact(second, pipeline)

				So(err, ShouldBeIn, nil, io.EOF)
				So(datura.Peek[float64](second, "output", "value"), ShouldBeGreaterThan, 0)

				relative := datura.Peek[float64](second, "output", "value")

				So(relative, ShouldBeGreaterThan, 0)
				So(math.IsNaN(relative), ShouldBeFalse)
				So(math.IsInf(relative, 0), ShouldBeFalse)
			})
		})
	})
}

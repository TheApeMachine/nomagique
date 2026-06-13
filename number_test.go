package nomagique_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/adaptive"
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/geometry"
	"github.com/theapemachine/nomagique/learning"
	"github.com/theapemachine/nomagique/probability"
)

func TestNumber(testingTB *testing.T) {
	Convey("Given a number", testingTB, func() {
		number, err := nomagique.Number(
			adaptive.EMA(),
			adaptive.Delta(),
		)
		So(err, ShouldBeNil)

		_ = number.Observe(adaptive.EMA(), adaptive.Delta())

		number += 1.0

		Convey("It should return a number", func() {
			So(number, ShouldEqual, 1)
		})
	})

	Convey("Given a retained adaptive number", testingTB, func() {
		exponential := adaptive.EMA()
		number, err := nomagique.Number(exponential)
		So(err, ShouldBeNil)

		Convey("When the scalar observes through the same dynamic", func() {
			number += 10
			_ = number.Observe(exponential)

			Convey("It should use the retained adaptive state", func() {
				So(number, ShouldEqual, 10)
			})
		})
	})

	Convey("Given nested adaptive numbers", testingTB, func() {
		chain, err := nomagique.Number(adaptive.EMA(), adaptive.Delta())
		So(err, ShouldBeNil)

		number, err := nomagique.Number(chain)
		So(err, ShouldBeNil)

		Convey("When the scalar observes through the nested chain", func() {
			number += 10

			var derived core.Float64
			derived = number.Observe(chain)
			number = nomagique.Scalar(derived)

			Convey("It should take the downstream derived value", func() {
				So(number, ShouldEqual, 1)
			})
		})
	})

	Convey("Given an EMA number driven by explicit raw samples", testingTB, func() {
		exponential := adaptive.EMA()
		number, err := nomagique.Number(exponential)
		So(err, ShouldBeNil)

		series := []float64{10, 5, 20, 20, 15}
		var last core.Float64

		for _, sample := range series {
			number = nomagique.Scalar(sample)

			last = number.Observe(exponential)
		}

		Convey("It should end inside the observed range", func() {
			So(float64(last), ShouldBeBetween, 5.0, 20.0)
		})
	})

	Convey("Given an EMA number fed by cumulative +=", testingTB, func() {
		exponential := adaptive.EMA()
		number, err := nomagique.Number(exponential)
		So(err, ShouldBeNil)

		Convey("When a sample is accumulated then observed", func() {
			number += 10

			var derived core.Float64
			derived = number.Observe(exponential)
			number = nomagique.Scalar(derived)

			Convey("It should treat the accumulated scalar as raw input", func() {
				So(number, ShouldEqual, 10)
			})
		})
	})

	Convey("Given an EMA then delta number over a sample series", testingTB, func() {
		exponential := adaptive.EMA()
		delta := adaptive.Delta()
		samples := []float64{10, 20, 5, 15}

		outputs, err := observeSamplesThroughNumber(
			[]core.Number{exponential, delta},
			samples,
		)
		So(err, ShouldBeNil)

		reference := referenceEMADeltaSeries(samples)

		Convey("It should match the fused adaptive kernel on every step", func() {
			So(outputs, ShouldResemble, reference)
		})
	})

	Convey("Given reversed EMA and delta stage order", testingTB, func() {
		samples := []float64{10, 15}

		forward, err := observeSamplesThroughNumber(
			[]core.Number{adaptive.EMA(), adaptive.Delta()},
			samples,
		)
		So(err, ShouldBeNil)

		reversed, err := observeSamplesThroughNumber(
			[]core.Number{adaptive.Delta(), adaptive.EMA()},
			samples,
		)
		So(err, ShouldBeNil)

		reference := referenceEMADeltaSeries(samples)

		Convey("It should match the canonical EMA-then-delta ordering", func() {
			So(forward, ShouldResemble, reference)
			So(reversed, ShouldResemble, reference)
		})
	})

	Convey("Given a nested chain over multiple samples", testingTB, func() {
		exponential := adaptive.EMA()
		delta := adaptive.Delta()
		chain, err := nomagique.Number(exponential, delta)
		So(err, ShouldBeNil)

		number, err := nomagique.Number(chain)
		So(err, ShouldBeNil)

		samples := []float64{10, 20, 5}
		nestedOutputs := make([]float64, len(samples))

		for index, sample := range samples {
			number = nomagique.Scalar(sample)

			var derived core.Float64
			derived = number.Observe(chain)

			nestedOutputs[index] = float64(derived)
		}

		reference := referenceEMADeltaSeries(samples)

		directOutputs, err := observeSamplesThroughNumber(
			[]core.Number{adaptive.EMA(), adaptive.Delta()},
			samples,
		)
		So(err, ShouldBeNil)

		Convey("It should match observing the flattened stages directly", func() {
			So(nestedOutputs, ShouldResemble, reference)
			So(directOutputs, ShouldResemble, reference)
		})
	})

	Convey("Given a deeply nested boundary token", testingTB, func() {
		exponential := adaptive.EMA()
		inner, err := nomagique.Number(exponential)
		So(err, ShouldBeNil)

		middle, err := nomagique.Number(inner)
		So(err, ShouldBeNil)

		outer, err := nomagique.Number(middle)
		So(err, ShouldBeNil)

		Convey("When observing through the outermost token", func() {
			outer = nomagique.Scalar(12)

			var derived core.Float64
			derived = outer.Observe(outer)

			reference := adaptive.EMA()
			_ = reference.ObserveSample(0)
			expect := reference.ObserveSample(12)

			Convey("It should run the registered EMA stages", func() {
				So(float64(derived), ShouldEqual, expect)
			})
		})
	})

	Convey("Given distinct EMA instances at observe time", testingTB, func() {
		retained := adaptive.EMA()
		number, err := nomagique.Number(retained)
		So(err, ShouldBeNil)

		number = nomagique.Scalar(10)

		var retainedOut core.Float64
		retainedOut = number.Observe(retained)

		var freshOut core.Float64
		freshOut = number.Observe(adaptive.EMA())

		Convey("It should not reuse state from an unregistered instance", func() {
			So(retainedOut, ShouldEqual, 10)

			number = nomagique.Scalar(5)
			retainedOut = number.Observe(retained)

			So(retainedOut, ShouldBeLessThan, 10)
			So(freshOut, ShouldEqual, 10)
		})
	})

	Convey("Given a delta-only number", testingTB, func() {
		delta := adaptive.Delta()
		number, err := nomagique.Number(delta)
		So(err, ShouldBeNil)

		Convey("When stepping through a range", func() {
			number = nomagique.Scalar(0)
			_ = number.Observe(delta)

			number = nomagique.Scalar(10)

			var derived core.Float64
			derived = number.Observe(delta)
			number = nomagique.Scalar(derived)

			Convey("It should emit a unit normalized step", func() {
				So(number, ShouldEqual, 1)
			})
		})
	})

	Convey("Given a two-stage observe and a core pipeline", testingTB, func() {
		exponential := adaptive.EMA()
		delta := adaptive.Delta()
		number, err := nomagique.Number(exponential, delta)
		So(err, ShouldBeNil)

		number = nomagique.Scalar(10)

		var fused core.Float64
		fused = number.Observe(exponential, delta)

		pipeline := core.AcquirePipeline([]core.Number{exponential, delta})
		_, err = pipeline.Observe(core.Float64(0))
		So(err, ShouldBeNil)

		var pipelined core.Float64
		pipelined, err = pipeline.Observe(core.Float64(10))
		So(err, ShouldBeNil)
		core.ReleasePipeline(pipeline)

		Convey("It should match the pooled pipeline path", func() {
			So(fused, ShouldEqual, pipelined)
		})
	})

	Convey("Given a boundary scalar without Number()", testingTB, func() {
		exponential := adaptive.EMA()
		delta := adaptive.Delta()
		raw := nomagique.Scalar(10)

		derived := raw.Observe(exponential, delta)

		reference := adaptive.EMA()
		deltaReference := adaptive.Delta()
		expect := adaptive.ObserveEMAThenDelta(10, reference, deltaReference)

		Convey("It should run the fused fast path on fresh dynamics", func() {
			So(float64(derived), ShouldEqual, expect)
		})
	})

	Convey("Given a slow-path observe with an extra stage", testingTB, func() {
		exponential := adaptive.EMA()
		delta := adaptive.Delta()
		tailExponential := adaptive.EMA()
		stages := []core.Number{exponential, delta, tailExponential}

		number, err := nomagique.Number(stages...)
		So(err, ShouldBeNil)

		number = nomagique.Scalar(10)

		var derived core.Float64
		derived = number.Observe(stages...)

		referenceExponential := adaptive.EMA()
		referenceDelta := adaptive.Delta()
		referenceTail := adaptive.EMA()
		referenceStages := []core.Number{
			referenceExponential, referenceDelta, referenceTail,
		}

		pipeline := core.AcquirePipeline(referenceStages)
		_, err = pipeline.Observe(core.Float64(0))
		So(err, ShouldBeNil)

		var pipelined core.Float64
		pipelined, err = pipeline.Observe(core.Float64(10))
		So(err, ShouldBeNil)
		core.ReleasePipeline(pipeline)

		Convey("It should match the generic pipeline ordering", func() {
			So(derived, ShouldEqual, pipelined)
		})
	})

	Convey("Given an EMA then z-score surprise chain", testingTB, func() {
		exponential := adaptive.EMA()
		surprise := adaptive.ZScore()
		number, err := nomagique.Number(exponential, surprise)
		So(err, ShouldBeNil)

		number = nomagique.Scalar(0)
		_ = number.Observe(exponential, surprise)

		number = nomagique.Scalar(10)

		var derived core.Float64
		derived = number.Observe(exponential, surprise)

		referenceExponential := adaptive.EMA()
		referenceSurprise := adaptive.ZScore()
		_ = referenceExponential.ObserveSample(0)
		_ = referenceSurprise.ObserveSample(0)
		reference := adaptive.ObserveEMAThenZScore(
			10, referenceExponential, referenceSurprise,
		)

		Convey("It should match the fused EMA-then-z-score kernel", func() {
			So(float64(derived), ShouldEqual, reference)
		})
	})

	Convey("Given a turbulence-style composition", testingTB, func() {
		chain, err := nomagique.Number(
			adaptive.FracDiff(),
			adaptive.Momentum(),
			adaptive.Compression(),
		)

		number, err := nomagique.Number(chain)
		So(err, ShouldBeNil)

		number = nomagique.Scalar(100)
		_ = number.Observe(chain)

		number = nomagique.Scalar(50)

		var derived core.Float64
		derived = number.Observe(chain)
		number = nomagique.Scalar(derived)

		Convey("It should emit a tightening compression score", func() {
			So(float64(number), ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given forecast scale feedback on settled outcomes", testingTB, func() {
		exponential := adaptive.EMA()
		forecaster := learning.Forecast()
		number, err := nomagique.Number(exponential)
		So(err, ShouldBeNil)

		number = nomagique.Scalar(10)
		level := number.Observe(exponential)

		_ = forecaster.Observe(level, core.Float64(10))
		_ = forecaster.Observe(core.Float64(10), core.Float64(15))

		Convey("It should learn a multiplicative scale", func() {
			So(forecaster.Scale(), ShouldBeGreaterThan, 1)
		})
	})

	Convey("Given EMA then phase velocity in a pipeline", testingTB, func() {
		exponential := adaptive.EMA()
		phaseVelocity := geometry.Velocity()
		number, err := nomagique.Number(exponential, phaseVelocity)
		So(err, ShouldBeNil)

		number = nomagique.Scalar(10)
		_ = number.Observe(exponential, phaseVelocity)

		number = nomagique.Scalar(12.5)

		var derived core.Float64
		derived = number.Observe(exponential, phaseVelocity)

		Convey("It should emit non-zero velocity after a level change", func() {
			So(float64(derived), ShouldNotEqual, 0)
		})
	})

	Convey("Given probability dynamics on a residual stream", testingTB, func() {
		changeSum := probability.CUSUM()
		empirical := probability.Rank()
		number, err := nomagique.Number(changeSum, empirical)
		So(err, ShouldBeNil)

		number = nomagique.Scalar(10)
		_ = number.Observe(changeSum, empirical)

		number = nomagique.Scalar(25)

		var derived core.Float64
		derived = number.Observe(changeSum, empirical)

		Convey("It should emit change evidence and rank probability", func() {
			So(float64(derived), ShouldBeGreaterThan, 0)
			So(float64(derived), ShouldBeLessThanOrEqualTo, 1)
		})
	})
}

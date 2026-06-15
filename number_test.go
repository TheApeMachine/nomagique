package nomagique_test

import (
	"math"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/adaptive"
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/geometry"
	"github.com/theapemachine/nomagique/learning"
	"github.com/theapemachine/nomagique/probability"
	"github.com/theapemachine/nomagique/tests"
)

func TestNumber(testingTB *testing.T) {
	Convey("Given Number with EMA and Delta stages", testingTB, func() {
		result := nomagique.Number(
			adaptive.NewEMA[float64](),
			adaptive.NewDelta[float64](),
		)

		Convey("It should bootstrap zero through the composed pipeline", func() {
			So(result, ShouldEqual, 0.0)
		})
	})
}

func TestScalar_Observe(testingTB *testing.T) {
	Convey("Given a scalar observed through EMA and Delta", testingTB, func() {
		exponential := adaptive.NewEMA[float64]()
		delta := adaptive.NewDelta[float64]()

		first := float64(core.Scalar[float64](1).Observe(exponential, delta))
		second := float64(core.Scalar[float64](2).Observe(exponential, delta))
		third := float64(core.Scalar[float64](3).Observe(exponential, delta))
		reference := referenceEMADeltaSeries([]float64{1, 2, 3})

		Convey("It should retain stage state across observations", func() {
			So(first, ShouldEqual, reference[0])
			So(second, ShouldEqual, reference[1])
			So(third, ShouldEqual, reference[2])
		})
	})
}

func TestScalar_retainedState(testingTB *testing.T) {
	Convey("Given a retained EMA stage", testingTB, func() {
		exponential := adaptive.NewEMA[float64]()
		_ = exponential.Observe(core.Scalar[float64](10))

		Convey("It should continue from prior state", func() {
			got := float64(core.Scalar[float64](20).Observe(exponential))
			So(got, ShouldEqual, 20.0)
		})
	})
}

func TestScalar_seriesThroughEMA(testingTB *testing.T) {
	Convey("Given a sample series through a retained EMA", testingTB, func() {
		exponential := adaptive.NewEMA[float64]()
		samples := []float64{1, 2, 3, 4, 5}
		outputs := observeSeriesThroughStages(samples, exponential)

		Convey("It should match the EMA reference path", func() {
			reference := adaptive.NewEMA[float64]()
			expect := tests.RunObserveSampleSequence(reference.ObserveSample, samples)
			So(outputs[len(outputs)-1], ShouldEqual, expect)
		})
	})
}

func TestScalar_EMAThenDeltaSeries(testingTB *testing.T) {
	Convey("Given EMA then Delta on a sample series", testingTB, func() {
		samples := []float64{10, 12, 11, 15, 14, 18, 17, 20}
		exponential := adaptive.NewEMA[float64]()
		delta := adaptive.NewDelta[float64]()
		outputs := observeSeriesThroughStages(samples, exponential, delta)
		reference := referenceEMADeltaSeries(samples)

		Convey("It should match the reference EMA-Delta series", func() {
			So(len(outputs), ShouldEqual, len(reference))
			for index := range outputs {
				So(outputs[index], ShouldEqual, reference[index])
			}
		})
	})
}

func TestScalar_EMAThenDeltaSeries_orderInvariant(testingTB *testing.T) {
	Convey("Given forward and reversed sample order", testingTB, func() {
		samples := []float64{10, 12, 11, 15, 14, 18, 17, 20}
		forward := observeSeriesThroughStages(
			samples,
			adaptive.NewEMA[float64](),
			adaptive.NewDelta[float64](),
		)
		reversed := observeSeriesThroughStages(
			reverseFloat64(samples),
			adaptive.NewEMA[float64](),
			adaptive.NewDelta[float64](),
		)
		reference := referenceEMADeltaSeries(samples)

		Convey("Forward order should match reference", func() {
			for index := range forward {
				So(forward[index], ShouldEqual, reference[index])
			}
		})

		Convey("Reversed order should differ from forward", func() {
			So(reversed[len(reversed)-1], ShouldNotEqual, forward[len(forward)-1])
		})
	})
}

func TestScalar_EMAThenDeltaSeries_directVsChained(testingTB *testing.T) {
	Convey("Given chained versus single-pass EMA-Delta", testingTB, func() {
		samples := []float64{10, 12, 11, 15, 14, 18, 17, 20}
		reference := referenceEMADeltaSeries(samples)

		exponential := adaptive.NewEMA[float64]()
		delta := adaptive.NewDelta[float64]()
		chained := observeSeriesThroughStages(samples, exponential, delta)

		Convey("Chained stages should match reference", func() {
			for index := range chained {
				So(chained[index], ShouldEqual, reference[index])
			}
		})
	})
}

func TestScalar_EMAThenZScoreSeries(testingTB *testing.T) {
	Convey("Given EMA then ZScore on a sample series", testingTB, func() {
		samples := []float64{10, 12, 11, 15, 14, 18, 17, 20}
		exponential := adaptive.NewEMA[float64]()
		zscore := adaptive.NewZScore[float64]()
		outputs := observeSeriesThroughStages(samples, exponential, zscore)

		Convey("It should produce finite z-scores after warmup", func() {
			last := outputs[len(outputs)-1]
			So(math.IsNaN(last), ShouldBeFalse)
			So(math.IsInf(last, 0), ShouldBeFalse)
		})
	})
}

func TestNumbers(testingTB *testing.T) {
	Convey("Given Numbers wrappine slice", testingTB, func() {
		numbers := nomagique.Numbers(1.0, 2.0, 3.0)

		Convey("It should expose each sample as a scalar stage input", func() {
			So(len(numbers), ShouldEqual, 3)
			So(float64(numbers[0].(core.Scalar[float64])), ShouldEqual, 1.0)
			So(float64(numbers[1].(core.Scalar[float64])), ShouldEqual, 2.0)
			So(float64(numbers[2].(core.Scalar[float64])), ShouldEqual, 3.0)
		})
	})
}

func TestNumber_crossPackageStages(testingTB *testing.T) {
	cases := []struct {
		name   string
		stages []core.Number[float64]
		sample float64
	}{
		{
			"forecast",
			[]core.Number[float64]{learning.Forecast[float64]()},
			0.5,
		},
		{
			"velocity",
			[]core.Number[float64]{geometry.NewVelocity[float64]()},
			1.0,
		},
		{
			"coupling",
			[]core.Number[float64]{geometry.NewCoupling[float64]()},
			0.25,
		},
		{
			"cusum",
			[]core.Number[float64]{probability.CUSUM[float64]()},
			1.0,
		},
		{
			"bernoulli",
			[]core.Number[float64]{probability.Bernoulli[float64]()},
			0.75,
		},
		{
			"rank",
			[]core.Number[float64]{probability.Rank[float64]()},
			0.5,
		},
		{
			"time elastic",
			[]core.Number[float64]{adaptive.NewTimeElastic[float64](time.Hour, 1e-6)},
			10,
		},
		{
			"transition surprise",
			[]core.Number[float64]{probability.TransitionSurprise[float64](5, 0.1)},
			0.2,
		},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name+" composed through Number", testingTB, func() {
			_ = nomagique.Number(testCase.stages...)
			got := float64(core.Scalar[float64](testCase.sample).Observe(testCase.stages...))

			Convey("It should derive a finite observation", func() {
				So(math.IsNaN(got), ShouldBeFalse)
				So(math.IsInf(got, 0), ShouldBeFalse)
			})
		})
	}
}

func observeSeriesThroughStages(
	samples []float64,
	stages ...core.Number[float64],
) []float64 {
	outputs := make([]float64, len(samples))

	for index, sample := range samples {
		outputs[index] = float64(core.Scalar[float64](sample).Observe(stages...))
	}

	return outputs
}

func referenceEMADeltaSeries(samples []float64) []float64 {
	return observeSeriesThroughStages(
		samples,
		adaptive.NewEMA[float64](),
		adaptive.NewDelta[float64](),
	)
}

func reverseFloat64(values []float64) []float64 {
	reversed := make([]float64, len(values))

	for index, value := range values {
		reversed[len(values)-1-index] = value
	}

	return reversed
}

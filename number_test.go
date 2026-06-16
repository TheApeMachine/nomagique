package nomagique_test

import (
	"io"
	"math"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/adaptive"
	"github.com/theapemachine/nomagique/geometry"
	"github.com/theapemachine/nomagique/learning"
	"github.com/theapemachine/nomagique/logic"
	"github.com/theapemachine/nomagique/probability"
	"github.com/theapemachine/nomagique/statistic"
	"github.com/theapemachine/nomagique/tests"
)

func TestNumber(testingTB *testing.T) {
	Convey("Given Number with EMA and Delta stages", testingTB, func() {
		exponential := adaptive.NewEMA()
		delta := adaptive.NewDelta()

		So(nomagique.Number(exponential, delta), ShouldBeNil)

		got, err := tests.PipelineSample([]io.ReadWriter{exponential, delta}, 0)

		So(err, ShouldBeNil)

		Convey("It should bootstrap zero through the composed pipeline", func() {
			So(got, ShouldEqual, 0.0)
		})
	})
}

func TestPipeline_retainedState(testingTB *testing.T) {
	Convey("Given a retained EMA stage", testingTB, func() {
		exponential := adaptive.NewEMA()
		_, _ = tests.PipelineSample([]io.ReadWriter{exponential}, 10)

		Convey("It should continue from prior state", func() {
			got, err := tests.PipelineSample([]io.ReadWriter{exponential}, 20)

			So(err, ShouldBeNil)
			So(got, ShouldEqual, 20.0)
		})
	})
}

func TestPipeline_seriesThroughEMA(testingTB *testing.T) {
	Convey("Given a sample series through a retained EMA", testingTB, func() {
		exponential := adaptive.NewEMA()
		samples := []float64{1, 2, 3, 4, 5}
		outputs, err := tests.PipelineSeries([]io.ReadWriter{exponential}, samples)

		So(err, ShouldBeNil)

		Convey("It should match the EMA reference path", func() {
			reference, referenceErr := tests.PipelineSeries([]io.ReadWriter{adaptive.NewEMA()}, samples)

			So(referenceErr, ShouldBeNil)
			So(outputs[len(outputs)-1], ShouldEqual, reference[len(reference)-1])
		})
	})
}

func TestPipeline_EMAThenDeltaSeries(testingTB *testing.T) {
	Convey("Given EMA then Delta on a sample series", testingTB, func() {
		samples := []float64{10, 12, 11, 15, 14, 18, 17, 20}
		exponential := adaptive.NewEMA()
		delta := adaptive.NewDelta()
		stages := []io.ReadWriter{exponential, delta}
		outputs, err := tests.PipelineSeries(stages, samples)
		reference, referenceErr := tests.PipelineSeries(
			[]io.ReadWriter{adaptive.NewEMA(), adaptive.NewDelta()},
			samples,
		)

		So(err, ShouldBeNil)
		So(referenceErr, ShouldBeNil)

		Convey("It should match the reference EMA-Delta series", func() {
			So(len(outputs), ShouldEqual, len(reference))
			for index := range outputs {
				So(outputs[index], ShouldEqual, reference[index])
			}
		})
	})
}

func TestPipeline_EMAThenDeltaSeries_orderInvariant(testingTB *testing.T) {
	Convey("Given forward and reversed sample order", testingTB, func() {
		samples := []float64{10, 12, 11, 15, 14, 18, 17, 20}
		forward, _ := tests.PipelineSeries(
			[]io.ReadWriter{adaptive.NewEMA(), adaptive.NewDelta()},
			samples,
		)
		reversed, _ := tests.PipelineSeries(
			[]io.ReadWriter{adaptive.NewEMA(), adaptive.NewDelta()},
			reverseFloat64(samples),
		)

		Convey("Reversed order should differ from forward", func() {
			So(reversed[len(reversed)-1], ShouldNotEqual, forward[len(reversed)-1])
		})
	})
}

func TestPipeline_EMAThenZScoreSeries(testingTB *testing.T) {
	Convey("Given EMA then ZScore on a sample series", testingTB, func() {
		samples := []float64{10, 12, 11, 15, 14, 18, 17, 20}
		outputs, err := tests.PipelineSeries(
			[]io.ReadWriter{adaptive.NewEMA(), adaptive.NewZScore()},
			samples,
		)

		So(err, ShouldBeNil)

		Convey("It should produce finite z-scores after warmup", func() {
			last := outputs[len(outputs)-1]
			So(math.IsNaN(last), ShouldBeFalse)
			So(math.IsInf(last, 0), ShouldBeFalse)
		})
	})
}

func TestNumber_PanelMedian(testingTB *testing.T) {
	Convey("Given Through and an explicit panel registry", testingTB, func() {
		panel := statistic.NewPanel()
		median := statistic.NewMedian(nil, panel)

		_ = tests.WriteSamples(panel, 1, 0.02)
		_, _ = tests.ReadSample(panel)
		_ = tests.WriteSamples(panel, 2, 0.04)
		_, _ = tests.ReadSample(panel)
		_ = tests.WriteSamples(panel, 3, 0.06)
		_, _ = tests.ReadSample(panel)

		got, err := tests.ReadSample(median)

		So(err, ShouldBeNil)

		Convey("It should return the peer median", func() {
			So(got, ShouldEqual, 0.05)
		})
	})
}

func TestConstants(testingTB *testing.T) {
	Convey("Given constant stages wrapping a slice", testingTB, func() {
		stages := []io.ReadWriter{
			logic.NewConstant(1.0),
			logic.NewConstant(2.0),
			logic.NewConstant(3.0),
		}

		Convey("It should expose each value as a constant stage", func() {
			So(len(stages), ShouldEqual, 3)

			first, _ := tests.ReadSample(stages[0])
			second, _ := tests.ReadSample(stages[1])
			third, _ := tests.ReadSample(stages[2])

			So(first, ShouldEqual, 1.0)
			So(second, ShouldEqual, 2.0)
			So(third, ShouldEqual, 3.0)
		})
	})
}

func TestNumber_crossPackageStages(testingTB *testing.T) {
	cases := []struct {
		name   string
		stages []io.ReadWriter
		sample float64
	}{
		{"forecast", []io.ReadWriter{learning.Forecast()}, 0.5},
		{"velocity", []io.ReadWriter{geometry.NewVelocity()}, 1.0},
		{"coupling", []io.ReadWriter{geometry.NewCoupling()}, 0.25},
		{"cusum", []io.ReadWriter{probability.NewCUSUM()}, 1.0},
		{"bernoulli", []io.ReadWriter{probability.NewBernoulli()}, 0.75},
		{"rank", []io.ReadWriter{probability.NewRank()}, 0.5},
		{"time elastic", []io.ReadWriter{adaptive.NewTimeElastic(time.Hour, 1e-6)}, 10},
		{"transition surprise", []io.ReadWriter{probability.NewTransitionSurprise(5, 0.1)}, 0.2},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name+" composed through Number", testingTB, func() {
			So(nomagique.Number(testCase.stages...), ShouldBeNil)

			got, err := tests.PipelineSample(testCase.stages, testCase.sample)

			So(err, ShouldBeNil)

			Convey("It should derive a finite observation", func() {
				So(math.IsNaN(got), ShouldBeFalse)
				So(math.IsInf(got, 0), ShouldBeFalse)
			})
		})
	}
}

func reverseFloat64(values []float64) []float64 {
	reversed := make([]float64, len(values))

	for index, value := range values {
		reversed[len(values)-1-index] = value
	}

	return reversed
}

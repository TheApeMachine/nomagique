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

func TestNumber_stress_singleAdaptiveDynamics(testingTB *testing.T) {
	samples := stressSamples()

	cases := []struct {
		name    string
		stageFn func() core.Number
	}{
		{name: "accumulator", stageFn: func() core.Number { return adaptive.Accumulator() }},
		{name: "compression", stageFn: func() core.Number { return adaptive.Compression() }},
		{name: "fracdiff", stageFn: func() core.Number { return adaptive.FracDiff() }},
		{name: "variance", stageFn: func() core.Number { return adaptive.Variance() }},
		{name: "momentum", stageFn: func() core.Number { return adaptive.Momentum() }},
		{name: "range", stageFn: func() core.Number { return adaptive.Range() }},
	}

	for _, testCase := range cases {
		testingTB.Run(testCase.name, func(runTestingTB *testing.T) {
			Convey("Given "+testCase.name+" through Number", runTestingTB, func() {
				stage := testCase.stageFn()
				stages := []core.Number{stage}
				outputs, err := observeSamplesThroughNumber(stages, samples)
				So(err, ShouldBeNil)

				referenceStage := testCase.stageFn()
				expect, err := referenceNumberAlignedSeries(
					[]core.Number{referenceStage}, samples,
				)

				Convey("It should be repeatable on a fresh instance", func() {
					So(outputs, ShouldResemble, expect)
					So(allFinite(outputs), ShouldBeTrue)
				})
			})
		})
	}
}

func TestNumber_stress_fusedAndTurbulenceSeries(testingTB *testing.T) {
	samples := stressSamples()

	Convey("Given EMA then z-score over a stress series", testingTB, func() {
		stages := []core.Number{adaptive.EMA(), adaptive.ZScore()}
		outputs, err := observeSamplesThroughNumber(stages, samples)
		So(err, ShouldBeNil)

		Convey("It should match the fused kernel on every step", func() {
			So(outputs, ShouldResemble, referenceEMAZScoreSeries(samples))
		})
	})

	Convey("Given a turbulence chain over a stress series", testingTB, func() {
		turbulenceStages := func() []core.Number {
			return []core.Number{
				adaptive.FracDiff(),
				adaptive.Momentum(),
				adaptive.Compression(),
			}
		}
		flat, err := observeSamplesThroughNumber(turbulenceStages(), samples)
		So(err, ShouldBeNil)

		nestedStages := turbulenceStages()
		chain, err := nomagique.Number(nestedStages...)
		So(err, ShouldBeNil)

		number, err := nomagique.Number(chain)
		So(err, ShouldBeNil)

		nested := make([]float64, len(samples))

		for index, sample := range samples {
			number = nomagique.Scalar(sample)

			var derived core.Float64
			derived = number.Observe(chain)

			nested[index] = float64(derived)
		}

		repeat, err := referenceNumberAlignedSeries(turbulenceStages(), samples)
		So(err, ShouldBeNil)

		Convey("It should agree across nested and repeated runs", func() {
			So(flat, ShouldResemble, nested)
			So(flat, ShouldResemble, repeat)
			So(allFinite(flat), ShouldBeTrue)
		})
	})
}

func TestNumber_stress_longMixedPipeline(testingTB *testing.T) {
	samples := stressSamples()
	mixedStages := func() []core.Number {
		return []core.Number{
			adaptive.EMA(),
			adaptive.Delta(),
			adaptive.Momentum(),
			adaptive.Compression(),
			adaptive.Range(),
		}
	}

	Convey("Given a five-stage pipeline through Number", testingTB, func() {
		outputs, err := observeSamplesThroughNumber(mixedStages(), samples)
		So(err, ShouldBeNil)

		expect, err := referenceNumberAlignedSeries(mixedStages(), samples)
		So(err, ShouldBeNil)

		Convey("It should be repeatable on a fresh pipeline", func() {
			So(outputs, ShouldResemble, expect)
		})
	})
}

func TestNumber_stress_probabilityAndGeometry(testingTB *testing.T) {
	samples := stressSamples()

	Convey("Given CUSUM then rank on a stress series", testingTB, func() {
		probabilityStages := func() []core.Number {
			return []core.Number{probability.CUSUM(), probability.Rank()}
		}
		outputs, err := observeSamplesThroughNumber(probabilityStages(), samples)
		So(err, ShouldBeNil)

		expect, err := referenceNumberAlignedSeries(probabilityStages(), samples)
		So(err, ShouldBeNil)

		Convey("It should be repeatable on a fresh pipeline", func() {
			So(outputs, ShouldResemble, expect)
			So(allFinite(outputs), ShouldBeTrue)
		})
	})

	Convey("Given EMA then phase velocity on a stress series", testingTB, func() {
		geometryStages := func() []core.Number {
			return []core.Number{adaptive.EMA(), geometry.Velocity()}
		}
		outputs, err := observeSamplesThroughNumber(geometryStages(), samples)
		So(err, ShouldBeNil)

		expect, err := referenceNumberAlignedSeries(geometryStages(), samples)
		So(err, ShouldBeNil)

		Convey("It should be repeatable on a fresh pipeline", func() {
			So(outputs, ShouldResemble, expect)
		})
	})
}

func TestNumber_stress_learningWithSignalLevel(testingTB *testing.T) {
	Convey("Given forecast learning on an EMA level stream", testingTB, func() {
		exponential := adaptive.EMA()
		forecaster := learning.Forecast()
		number, err := nomagique.Number(exponential)
		So(err, ShouldBeNil)

		predicted := []float64{10, 12, 11, 15, 14}
		actual := []float64{10, 15, 9, 20, 13}

		for index := range predicted {
			number = nomagique.Scalar(predicted[index])

			var level core.Float64
			level = number.Observe(exponential)

			_ = forecaster.Observe(level, core.Float64(actual[index]))
		}

		Convey("It should finish with a usable scale", func() {
			So(forecaster.Scale(), ShouldBeGreaterThan, 0)
		})
	})
}

func TestNumber_stress_highVolumeTicks(testingTB *testing.T) {
	Convey("Given a long mixed pipeline", testingTB, func() {
		stages := []core.Number{
			adaptive.EMA(),
			adaptive.Delta(),
			adaptive.ZScore(),
			probability.CUSUM(),
			geometry.Velocity(),
		}
		number, err := nomagique.Number(stages...)
		So(err, ShouldBeNil)

		sample := 10.0

		for tick := 0; tick < 2048; tick++ {
			number = nomagique.Scalar(sample)

			var derived core.Float64
			derived = number.Observe(stages...)

			value := float64(derived)
			So(allFinite([]float64{value}), ShouldBeTrue)

			sample += 0.013
		}
	})
}

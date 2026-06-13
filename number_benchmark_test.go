package nomagique_test

import (
	"testing"

	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/adaptive"
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/geometry"
	"github.com/theapemachine/nomagique/learning"
	"github.com/theapemachine/nomagique/probability"
)

type numberBenchCase struct {
	name   string
	stages []core.Number
}

func prepareNumberBench(testingTB *testing.B, stages []core.Number) nomagique.Scalar {
	testingTB.Helper()

	number, err := nomagique.Number(stages...)

	if err != nil {
		testingTB.Fatal(err)
	}

	_ = number.Observe(stages...)

	return number
}

func BenchmarkNumber_construct(testingTB *testing.B) {
	stages := []core.Number{
		adaptive.EMA(),
		adaptive.Delta(),
		adaptive.ZScore(),
	}

	for testingTB.Loop() {
		_, err := nomagique.Number(stages...)

		if err != nil {
			testingTB.Fatal(err)
		}
	}
}

func BenchmarkNumber_retainedObserve(testingTB *testing.B) {
	for _, benchCase := range []numberBenchCase{
		{name: "ema", stages: []core.Number{adaptive.EMA()}},
		{name: "delta", stages: []core.Number{adaptive.Delta()}},
		{name: "accumulator", stages: []core.Number{adaptive.Accumulator()}},
		{name: "compression", stages: []core.Number{adaptive.Compression()}},
		{name: "fracdiff", stages: []core.Number{adaptive.FracDiff()}},
		{name: "variance", stages: []core.Number{adaptive.Variance()}},
		{name: "zscore", stages: []core.Number{adaptive.ZScore()}},
		{name: "momentum", stages: []core.Number{adaptive.Momentum()}},
		{name: "range", stages: []core.Number{adaptive.Range()}},
		{name: "ema_delta", stages: []core.Number{adaptive.EMA(), adaptive.Delta()}},
		{name: "ema_zscore", stages: []core.Number{adaptive.EMA(), adaptive.ZScore()}},
		{
			name: "turbulence",
			stages: []core.Number{
				adaptive.FracDiff(),
				adaptive.Momentum(),
				adaptive.Compression(),
			},
		},
		{name: "cusum_rank", stages: []core.Number{probability.CUSUM(), probability.Rank()}},
		{name: "ema_velocity", stages: []core.Number{adaptive.EMA(), geometry.Velocity()}},
		{
			name: "long_mixed",
			stages: []core.Number{
				adaptive.EMA(),
				adaptive.Delta(),
				adaptive.Momentum(),
				adaptive.Compression(),
				adaptive.Range(),
			},
		},
	} {
		testingTB.Run(benchCase.name, func(runTestingTB *testing.B) {
			number := prepareNumberBench(runTestingTB, benchCase.stages)

			for runTestingTB.Loop() {
				number = nomagique.Scalar(1.25)

				number.Observe(benchCase.stages...)
			}
		})
	}
}

func BenchmarkNumber_scalarFreshObserve(testingTB *testing.B) {
	for _, benchCase := range []numberBenchCase{
		{name: "ema_delta", stages: []core.Number{adaptive.EMA(), adaptive.Delta()}},
		{name: "ema_zscore", stages: []core.Number{adaptive.EMA(), adaptive.ZScore()}},
		{
			name: "turbulence",
			stages: []core.Number{
				adaptive.FracDiff(),
				adaptive.Momentum(),
				adaptive.Compression(),
			},
		},
	} {
		testingTB.Run(benchCase.name, func(runTestingTB *testing.B) {
			raw := nomagique.Scalar(1.25)

			for runTestingTB.Loop() {
				raw.Observe(benchCase.stages...)
			}
		})
	}
}

func BenchmarkNumber_nestedChain(testingTB *testing.B) {
	chain, err := nomagique.Number(
		adaptive.FracDiff(),
		adaptive.Momentum(),
		adaptive.Compression(),
	)

	if err != nil {
		testingTB.Fatal(err)
	}

	number := prepareNumberBench(testingTB, []core.Number{chain})

	for testingTB.Loop() {
		number = nomagique.Scalar(1.25)

		_ = number.Observe(chain)
	}
}

func BenchmarkNumber_slowPathObserve(testingTB *testing.B) {
	stages := []core.Number{
		adaptive.EMA(),
		adaptive.Delta(),
		adaptive.EMA(),
	}
	number := prepareNumberBench(testingTB, stages)

	for testingTB.Loop() {
		number = nomagique.Scalar(1.25)

		number.Observe(stages...)
	}
}

func BenchmarkNumber_stressSeries(testingTB *testing.B) {
	samples := stressSamples()
	stages := []core.Number{
		adaptive.EMA(),
		adaptive.Delta(),
		adaptive.ZScore(),
		probability.CUSUM(),
		geometry.Velocity(),
	}
	number := prepareNumberBench(testingTB, stages)
	sampleIndex := 0

	for testingTB.Loop() {
		number = nomagique.Scalar(samples[sampleIndex%len(samples)])

		derived := number.Observe(stages...)

		number = nomagique.Scalar(derived)
		sampleIndex++
	}
}

func BenchmarkNumber_learningForecast(testingTB *testing.B) {
	exponential := adaptive.EMA()
	forecaster := learning.Forecast()
	number := prepareNumberBench(testingTB, []core.Number{exponential})

	for testingTB.Loop() {
		number = nomagique.Scalar(10)

		level := number.Observe(exponential)

		_ = forecaster.Observe(level, core.Float64(12))
	}
}

func BenchmarkNumber_probabilityBernoulli(testingTB *testing.B) {
	posterior := probability.Bernoulli()

	for testingTB.Loop() {
		posterior.Observe(core.Float64(10), core.Float64(11))
	}
}

func BenchmarkNumber_geometricCoupling(testingTB *testing.B) {
	phaseCoupling := geometry.Coupling()

	for testingTB.Loop() {
		phaseCoupling.Observe(core.Float64(1.7), core.Float64(-0.9))
	}
}

func BenchmarkNumber_constructAndObserve(testingTB *testing.B) {
	for testingTB.Loop() {
		number, err := nomagique.Number(adaptive.EMA(), adaptive.Delta())

		if err != nil {
			testingTB.Fatal(err)
		}

		number += 1.0

		_ = number.Observe(adaptive.EMA(), adaptive.Delta())
	}
}

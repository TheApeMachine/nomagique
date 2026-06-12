package nomagique_test

import (
	"math"

	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/adaptive"
	"github.com/theapemachine/nomagique/core"
)

func stressSamples() []float64 {
	return []float64{
		10, 20, 5, 15, 100, 50, 25, 75,
		12, 12, 12, 40, 8, 90, 90, 10,
		3, 7, 11, 19, 23, 31, 2, 2,
		60, 55, 45, 48, 52, 49, 51, 50,
	}
}

func observeSamplesThroughNumber(
	stages []core.Number,
	samples []float64,
) ([]float64, error) {
	number, err := nomagique.Number(stages...)

	if err != nil {
		return nil, err
	}

	outputs := make([]float64, len(samples))

	for index, sample := range samples {
		number = nomagique.Scalar(sample)

		var derived core.Float64

		derived = number.Observe(stages...)

		outputs[index] = float64(derived)
	}

	return outputs, nil
}

func referenceEMADeltaSeries(samples []float64) []float64 {
	exponential := adaptive.EMA()
	delta := adaptive.Delta()
	_ = adaptive.ObserveEMAThenDelta(0, exponential, delta)

	reference := make([]float64, len(samples))

	for index, sample := range samples {
		reference[index] = adaptive.ObserveEMAThenDelta(sample, exponential, delta)
	}

	return reference
}

func referenceEMAZScoreSeries(samples []float64) []float64 {
	exponential := adaptive.EMA()
	surprise := adaptive.ZScore()
	_ = adaptive.ObserveEMAThenZScore(0, exponential, surprise)

	reference := make([]float64, len(samples))

	for index, sample := range samples {
		reference[index] = adaptive.ObserveEMAThenZScore(
			sample, exponential, surprise,
		)
	}

	return reference
}

func referenceNumberAlignedSeries(
	stages []core.Number, samples []float64,
) ([]float64, error) {
	return observeSamplesThroughNumber(stages, samples)
}

func allFinite(values []float64) bool {
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}

	return true
}

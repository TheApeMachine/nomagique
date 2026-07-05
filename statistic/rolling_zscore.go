package statistic

import (
	"math"

	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
RollingZScore normalizes the current sample against its retained series.
*/
type RollingZScore struct {
	samples map[string][]TimedSample
}

/*
NewRollingZScore returns a typed rolling z-score accumulator.
*/
func NewRollingZScore() *RollingZScore {
	return &RollingZScore{
		samples: map[string][]TimedSample{},
	}
}

/*
Measure adds one timed sample and returns its z-score against prior samples.
*/
func (rollingZScore *RollingZScore) Measure(sample TimedSample) (ScalarOutput, error) {
	if err := finiteStatistic("rolling-zscore", sample.Value); err != nil {
		return ScalarOutput{}, err
	}

	if sample.At.IsZero() {
		return ScalarOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"rolling-zscore: event timestamp required",
			nil,
		))
	}

	series := sample.Series

	if series == "" {
		series = "default"
	}

	history := rollingZScore.samples[series]

	if len(history) > 0 && sample.At.Before(history[len(history)-1].At) {
		return ScalarOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"rolling-zscore: event timestamp must not regress",
			nil,
		))
	}

	prior := make([]float64, len(history))

	for index, point := range history {
		prior[index] = point.Value
	}

	history = append(history, TimedSample{
		Series: series,
		Value:  sample.Value,
		At:     sample.At,
	})
	rollingZScore.samples[series] = history

	if len(prior) == 0 {
		return ScalarOutput{
			Ready: true,
			Count: len(history),
		}, nil
	}

	score := rollingScore(sample.Value, prior)

	return ScalarOutput{
		Value: score,
		Ready: true,
		Count: len(history),
	}, nil
}

func rollingScore(sample float64, prior []float64) float64 {
	meanSample := stat.Mean(prior, nil)
	stdSample := stat.StdDev(prior, nil)

	if stdSample > 0 && !math.IsNaN(stdSample) && !math.IsInf(stdSample, 0) {
		return (sample - meanSample) / stdSample
	}

	meanAbsoluteDeviation := 0.0

	for _, priorSample := range prior {
		meanAbsoluteDeviation += math.Abs(priorSample - meanSample)
	}

	meanAbsoluteDeviation /= float64(len(prior))
	delta := sample - meanSample

	if meanAbsoluteDeviation > 0 {
		return delta / meanAbsoluteDeviation
	}

	if delta == 0 {
		return 0
	}

	return delta / math.Abs(delta)
}

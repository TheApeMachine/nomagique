package statistic

import (
	"math"

	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
RollingZScoreConfig provides optional window hints for the retained
per-series history. LongHint bounds retained history directly. Without it,
history is passed through ResolveWindowSet the same way MeanMedianRatio
does — which resolves a window sized to whatever history already exists,
so it only trims once a caller supplies an explicit LongHint. A caller
feeding an indefinitely long, single-series stream (e.g. one book tick per
update over a live trading session) should set LongHint to keep retained
history and the per-sample mean/stddev recompute bounded.
*/
type RollingZScoreConfig struct {
	ShortHint int
	LongHint  int
}

/*
RollingZScore normalizes the current sample against its retained series.
*/
type RollingZScore struct {
	config  RollingZScoreConfig
	samples map[string][]TimedSample
}

/*
NewRollingZScore returns a typed rolling z-score accumulator.
*/
func NewRollingZScore(configs ...RollingZScoreConfig) *RollingZScore {
	config := RollingZScoreConfig{}

	if len(configs) > 0 {
		config = configs[0]
	}

	return &RollingZScore{
		config:  config,
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
	history = rollingZScore.trim(history)
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

func (rollingZScore *RollingZScore) trim(history []TimedSample) []TimedSample {
	longWindow := rollingZScore.config.LongHint

	if longWindow <= 0 {
		resolved, err := ResolveWindowSet(timedValues(history), WindowsConfig{
			ShortHint: rollingZScore.config.ShortHint,
		})

		if err == nil {
			longWindow = resolved.LongWindow
		}
	}

	if longWindow <= 0 || len(history) <= longWindow {
		return history
	}

	return history[len(history)-longWindow:]
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

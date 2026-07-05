package statistic

import (
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
FastSlowConfig configures the adaptive fast/slow comparison.
*/
type FastSlowConfig struct {
	FastWindow int
	Invert     bool
}

/*
FastSlow compares trailing fast-window mean rate to the preceding slow window.
*/
type FastSlow struct {
	config  FastSlowConfig
	history []float64
}

/*
NewFastSlow returns a typed fast/slow ratio accumulator.
*/
func NewFastSlow(configs ...FastSlowConfig) *FastSlow {
	config := FastSlowConfig{}

	if len(configs) > 0 {
		config = configs[0]
	}

	return &FastSlow{
		config: config,
	}
}

/*
Measure adds one non-negative sample and returns the fast/slow ratio when ready.
*/
func (fastSlow *FastSlow) Measure(sample float64) (ScalarOutput, error) {
	if err := finiteStatistic("fast-slow", sample); err != nil {
		return ScalarOutput{}, err
	}

	if sample < 0 {
		return ScalarOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"fast-slow: value must be non-negative",
			nil,
		))
	}

	fastSlow.history = append(fastSlow.history, sample)
	window, err := ResolveWindowSet(fastSlow.history, WindowsConfig{
		ShortHint: fastSlow.config.FastWindow,
	})

	if err != nil {
		return ScalarOutput{}, err
	}

	fastWindow := window.ShortWindow

	if len(fastSlow.history) <= fastWindow {
		return ScalarOutput{
			Ready: false,
			Count: len(fastSlow.history),
		}, nil
	}

	sampleCount := len(fastSlow.history)
	slowCount := sampleCount - fastWindow
	recentRate := stat.Mean(fastSlow.history[sampleCount-fastWindow:], nil)
	olderRate := stat.Mean(fastSlow.history[:slowCount], nil)

	if recentRate <= 0 || olderRate <= 0 {
		return ScalarOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"fast-slow: rates must be positive",
			nil,
		))
	}

	value := recentRate / olderRate

	if fastSlow.config.Invert {
		value = olderRate / recentRate
	}

	return ScalarOutput{
		Value: value,
		Ready: true,
		Count: len(fastSlow.history),
	}, nil
}

package adaptive

import (
	"github.com/theapemachine/errnie"
)

/*
EMA is an exponential moving average retained as a single O(1) recurrence.
*/
type EMA struct {
	period     int
	smoothing  float64
	alpha      float64
	value      float64
	initialized bool
}

/*
EMAConfig describes direct EMA parameters.
*/
type EMAConfig struct {
	Period    int
	Smoothing float64
}

/*
NewEMA returns a direct EMA calculator.
Period and smoothing jointly define the recurrence weight; both must be
positive so the stage never invents a hidden default kernel.
*/
func NewEMA(configs ...EMAConfig) (*EMA, error) {
	config := EMAConfig{}

	if len(configs) > 0 {
		config = configs[0]
	}

	if config.Period <= 0 {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"ema: positive period required",
			nil,
		))
	}

	if config.Smoothing <= 0 {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"ema: positive smoothing required",
			nil,
		))
	}

	return &EMA{
		period:    config.Period,
		smoothing: config.Smoothing,
		alpha:     config.Smoothing / float64(config.Period+1),
	}, nil
}

/*
Measure folds each sample into the retained EMA without allocating channels.
*/
func (ema *EMA) Measure(samples ...float64) (float64, error) {
	if ema == nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"ema: calculator required",
			nil,
		))
	}

	if len(samples) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"ema: samples required",
			nil,
		))
	}

	for _, sample := range samples {
		if err := finiteAdaptive("ema", sample); err != nil {
			return 0, err
		}

		if !ema.initialized {
			ema.value = sample
			ema.initialized = true

			continue
		}

		ema.value += ema.alpha * (sample - ema.value)
	}

	if err := finiteAdaptive("ema", ema.value); err != nil {
		return 0, err
	}

	return ema.value, nil
}

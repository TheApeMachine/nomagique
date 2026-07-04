package adaptive

import (
	"context"

	"github.com/cinar/indicator/v2/helper"
	"github.com/cinar/indicator/v2/trend"
	"github.com/theapemachine/errnie"
)

/*
EMA is an exponential moving average calculator backed by cinar/indicator.
*/
type EMA struct {
	inner *trend.Ema[float64]
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
*/
func NewEMA(configs ...EMAConfig) *EMA {
	config := EMAConfig{}

	if len(configs) > 0 {
		config = configs[0]
	}

	inner := trend.NewEma[float64]()

	if config.Period > 0 {
		inner.Period = config.Period
	}

	if config.Smoothing > 0 {
		inner.Smoothing = config.Smoothing
	}

	return &EMA{
		inner: inner,
	}
}

/*
Measure returns the latest EMA for the provided samples.
*/
func (ema *EMA) Measure(samples ...float64) (float64, error) {
	if len(samples) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"ema: samples required",
			nil,
		))
	}

	inputChannel := helper.SliceToChanWithContext(context.Background(), samples)
	outputChannel := ema.inner.ComputeWithContext(context.Background(), inputChannel)
	values := helper.ChanToSlice(outputChannel)

	if len(values) == 0 {
		return samples[len(samples)-1], nil
	}

	return values[len(values)-1], nil
}

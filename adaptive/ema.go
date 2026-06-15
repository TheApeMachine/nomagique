package adaptive

import (
	"math"

	"github.com/theapemachine/nomagique/core"
)

/*
EMA is a volatility-adaptive exponential moving average stage.
*/
type EMA[T ~float64] struct {
	Value  float64
	Prev   float64
	Min    float64
	Max    float64
	Rate   float64
	Ready  bool
	output core.Scalar[T]
}

/*
NewEMA returns an EMA stage ready to bootstrap from its first observation.
Optional initial stages compose on a zero boundary before the first sample.
*/
func NewEMA[T ~float64](initial ...core.Number[T]) *EMA[T] {
	ema := &EMA[T]{}

	if len(initial) > 0 {
		ema.output = core.Scalar[T](0).Observe(initial...)
	}

	return ema
}

/*
Observe absorbs the carried sample and evolves the EMA.
*/
func (ema *EMA[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if len(inputs) == 0 {
		return ema.output
	}

	sample, ok := inputs[0].(core.Scalar[T])

	if !ok {
		return ema.output
	}

	if len(inputs) > 1 {
		if work, workOK := inputs[1].(core.Scalar[T]); workOK {
			sample = core.Scalar[T](T(sample) + T(work))
		}
	}

	ema.output = core.Scalar[T](T(ema.observe(float64(sample))))

	return ema.output
}

/*
ObserveSample ingests one raw sample through the EMA kernel.
*/
func (ema *EMA[T]) ObserveSample(sample T) T {
	derived := T(ema.observe(float64(sample)))
	ema.output = core.Scalar[T](derived)

	return derived
}

/*
ObserveSamples writes one derived value per sample into out.
*/
func (ema *EMA[T]) ObserveSamples(samples []T, out []T) {
	for index, sample := range samples {
		out[index] = ema.ObserveSample(sample)
	}
}

/*
Reset clears derived state so the next Observe bootstraps again.
*/
func (ema *EMA[T]) Reset() error {
	ema.Value = 0
	ema.Prev = 0
	ema.Min = 0
	ema.Max = 0
	ema.Rate = 0
	ema.Ready = false
	ema.output = core.Scalar[T](0)

	return nil
}

func (ema *EMA[T]) observe(sample float64) float64 {
	if !ema.Ready {
		ema.Value = sample
		ema.Prev = sample
		ema.Min = sample
		ema.Max = sample
		ema.Ready = true

		return ema.Value
	}

	return ema.observeReady(sample)
}

func (ema *EMA[T]) observeReady(sample float64) float64 {
	ema.Min = math.Min(ema.Min, sample)
	ema.Max = math.Max(ema.Max, sample)

	span := ema.Max - ema.Min

	if span == 0 {
		ema.Prev = sample
		return ema.Value
	}

	delta := math.Abs(sample - ema.Prev)
	ema.Rate = delta / span
	ema.Value += ema.Rate * (sample - ema.Value)
	ema.Prev = sample

	return ema.Value
}

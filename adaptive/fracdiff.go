package adaptive

import (
	"math"

	"github.com/theapemachine/errnie"
)

/*
FracDiff applies a fractional differencing filter with a configured order.
*/
type FracDiff struct {
	history []float64
	weights []float64
	maxLag  int
	order   float64
	width   int
	head    int
	count   int
}

/*
FracDiffConfig controls bounded fractional-difference memory and kernel order.
*/
type FracDiffConfig struct {
	MaxLag           int
	Order            float64
	WeightThreshold  float64
}

/*
FracDiffOutput reports the latest fractional difference.
*/
type FracDiffOutput struct {
	Value float64
	Ready bool
	Count int
}

/*
NewFracDiff returns a typed fractional-difference tracker.
Order and weight threshold are fixed at construction so the kernel does not
reinterpret history when sample units change.
*/
func NewFracDiff(configs ...FracDiffConfig) (*FracDiff, error) {
	config := FracDiffConfig{}

	if len(configs) > 0 {
		config = configs[0]
	}

	if config.MaxLag <= 0 {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"fracdiff: positive max lag required",
			nil,
		))
	}

	if config.Order <= 0 || config.Order > 1 {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"fracdiff: order must be in (0,1]",
			nil,
		))
	}

	if config.WeightThreshold <= 0 {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"fracdiff: positive weight threshold required",
			nil,
		))
	}

	weights, width := buildFracDiffWeights(
		config.Order,
		config.WeightThreshold,
		nil,
		config.MaxLag,
	)

	return &FracDiff{
		history: make([]float64, config.MaxLag+1),
		weights: weights,
		maxLag:  config.MaxLag,
		order:   config.Order,
		width:   width,
	}, nil
}

/*
Measure adds one sample and returns the fractional difference when the kernel
has enough history to evaluate.
*/
func (fractional *FracDiff) Measure(sample float64) (FracDiffOutput, error) {
	if err := finiteAdaptive("fracdiff", sample); err != nil {
		return FracDiffOutput{}, err
	}

	fractional.pushHistory(sample)

	if fractional.count < fractional.width {
		return FracDiffOutput{
			Ready: false,
			Count: fractional.count,
		}, nil
	}

	value := fractional.outputSum()

	if err := finiteAdaptive("fracdiff", value); err != nil {
		return FracDiffOutput{}, err
	}

	return FracDiffOutput{
		Value: value,
		Ready: true,
		Count: fractional.count,
	}, nil
}

func (fractional *FracDiff) pushHistory(sample float64) {
	if fractional.count < len(fractional.history) {
		fractional.history[fractional.count] = sample
		fractional.head = fractional.count
		fractional.count++

		return
	}

	fractional.head = (fractional.head + 1) % len(fractional.history)
	fractional.history[fractional.head] = sample
	fractional.count++
}

func (fractional *FracDiff) outputSum() float64 {
	sum := 0.0

	for lag := 0; lag < fractional.width; lag++ {
		index := fractional.head - lag

		if index < 0 {
			index += len(fractional.history)
		}

		sum += fractional.weights[lag] * fractional.history[index]
	}

	return sum
}

/*
FractionalDifferenceValue applies a configured binomial fractional-difference
kernel to an already ordered sample series.
*/
func FractionalDifferenceValue(
	samples []float64,
	config FracDiffConfig,
) (float64, bool, error) {
	if config.MaxLag <= 0 {
		return 0, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"fracdiff: positive max lag required",
			nil,
		))
	}

	if config.Order <= 0 || config.Order > 1 {
		return 0, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"fracdiff: order must be in (0,1]",
			nil,
		))
	}

	if config.WeightThreshold <= 0 {
		return 0, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"fracdiff: positive weight threshold required",
			nil,
		))
	}

	weights, width := buildFracDiffWeights(
		config.Order,
		config.WeightThreshold,
		nil,
		config.MaxLag,
	)

	if len(samples) < width {
		return 0, false, nil
	}

	value := 0.0

	for lag := 0; lag < width; lag++ {
		sample := samples[len(samples)-1-lag]

		if math.IsNaN(sample) || math.IsInf(sample, 0) {
			return 0, false, errnie.Error(errnie.Err(
				errnie.Validation,
				"fracdiff: sample is non-finite",
				nil,
			))
		}

		value += weights[lag] * sample
	}

	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"fracdiff: output value is non-finite",
			nil,
		))
	}

	return value, true, nil
}

func buildFracDiffWeights(
	order float64,
	threshold float64,
	scratch []float64,
	maxLag int,
) ([]float64, int) {
	weights := scratch

	if cap(weights) < 1 {
		weights = make([]float64, 0, maxLag+1)
	}

	weights = weights[:1]
	weights[0] = 1
	weight := 1.0
	width := 1

	for lag := 1; lag <= maxLag; lag++ {
		weight = -weight * (order - float64(lag) + 1) / float64(lag)

		if math.Abs(weight) < threshold {
			return weights, width
		}

		weights = append(weights, weight)
		width++
	}

	return weights, width
}

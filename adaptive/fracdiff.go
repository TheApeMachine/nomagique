package adaptive

import (
	"math"

	"github.com/theapemachine/errnie"
)

const (
	defaultFracDiffMaxLag = 2048
)

/*
FracDiff applies a fractional differencing filter to recent samples.
*/
type FracDiff struct {
	history []float64
	weights []float64
	maxLag  int
	min     float64
	max     float64
	prev    float64
	order   float64
	width   int
	head    int
	count   int
	ready   bool
}

/*
FracDiffConfig controls bounded fractional-difference memory.
*/
type FracDiffConfig struct {
	MaxLag int
}

/*
FracDiffOutput reports the latest adaptive fractional difference.
*/
type FracDiffOutput struct {
	Value float64
	Ready bool
	Count int
}

/*
NewFracDiff returns a typed fractional-difference tracker.
*/
func NewFracDiff(configs ...FracDiffConfig) *FracDiff {
	config := FracDiffConfig{MaxLag: defaultFracDiffMaxLag}

	if len(configs) > 0 {
		config = configs[0]
	}

	if config.MaxLag <= 0 {
		config.MaxLag = defaultFracDiffMaxLag
	}

	return &FracDiff{
		maxLag: config.MaxLag,
	}
}

/*
Measure adds one sample and returns the adaptive fractional difference when ready.
*/
func (fractional *FracDiff) Measure(sample float64) (FracDiffOutput, error) {
	if err := finiteAdaptive("fracdiff", sample); err != nil {
		return FracDiffOutput{}, err
	}

	if !fractional.ready {
		capacity := fractional.capacity()
		fractional.history = make([]float64, capacity)
		fractional.history[0] = sample
		fractional.min = sample
		fractional.max = sample
		fractional.prev = sample
		fractional.order = 0
		fractional.width = 1
		fractional.head = 0
		fractional.count = 1
		fractional.weights = make([]float64, capacity)
		fractional.weights[0] = 1
		fractional.weights = fractional.weights[:1]
		fractional.ready = true

		return FracDiffOutput{
			Ready: false,
			Count: fractional.count,
		}, nil
	}

	fractional.min = math.Min(fractional.min, sample)
	fractional.max = math.Max(fractional.max, sample)
	span := fractional.max - fractional.min

	if span == 0 {
		fractional.pushHistory(sample)

		return FracDiffOutput{
			Ready: false,
			Count: fractional.count,
		}, nil
	}

	rate := math.Abs(sample-fractional.prev) / span
	order := fracDiffOrder(rate, span)
	smoothedOrder := order

	if fractional.count > 1 {
		smoothedOrder = 0.95*fractional.order + 0.05*order
	}

	if fractional.count == 1 || math.Abs(smoothedOrder-fractional.order) > 0.01 {
		fractional.rebuildWeights(smoothedOrder, span)
	}

	fractional.pushHistory(sample)
	fractional.prev = sample
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

func (fractional *FracDiff) rebuildWeights(order float64, span float64) {
	if order == fractional.order && fractional.width > 0 {
		return
	}

	fractional.order = order
	weights := fractional.weights[:0]
	weights, width := buildFracDiffWeights(
		order,
		span,
		fractional.prev,
		weights,
		fractional.maxLagForSpan(span),
	)
	fractional.width = width
	fractional.weights = weights[:width]
}

func (fractional *FracDiff) capacity() int {
	return fractional.configuredMaxLag() + 1
}

func (fractional *FracDiff) maxLagForSpan(span float64) int {
	maxLag := fractional.configuredMaxLag()
	lag := fracDiffMaxLag(span)

	if lag > maxLag {
		return maxLag
	}

	return lag
}

func (fractional *FracDiff) configuredMaxLag() int {
	if fractional.maxLag <= 0 {
		return defaultFracDiffMaxLag
	}

	return fractional.maxLag
}

func (fractional *FracDiff) pushHistory(sample float64) {
	if len(fractional.history) == 0 {
		return
	}

	fractional.head = (fractional.head + 1) % len(fractional.history)
	fractional.history[fractional.head] = sample

	if fractional.count < len(fractional.history) {
		fractional.count++
	}
}

func (fractional *FracDiff) outputSum() float64 {
	sum := 0.0
	limit := fractional.width

	if fractional.count < limit {
		limit = fractional.count
	}

	for lag := 0; lag < limit; lag++ {
		index := fractional.head - lag

		if index < 0 {
			index += len(fractional.history)
		}

		sum += fractional.weights[lag] * fractional.history[index]
	}

	return sum
}

/*
FractionalDifferenceValue applies the same binomial fractional-difference
kernel as FracDiff to an already normalized sample series.
*/
func FractionalDifferenceValue(samples []float64) (float64, bool, error) {
	if len(samples) < 3 {
		return 0, false, nil
	}

	minValue := samples[0]
	maxValue := samples[0]

	for _, sample := range samples {
		if math.IsNaN(sample) || math.IsInf(sample, 0) {
			return 0, false, errnie.Error(errnie.Err(
				errnie.Validation,
				"fracdiff: sample is non-finite",
				nil,
			))
		}

		minValue = math.Min(minValue, sample)
		maxValue = math.Max(maxValue, sample)
	}

	span := maxValue - minValue

	if span <= 0 {
		return 0, false, nil
	}

	tail := samples[len(samples)-1]
	prev := samples[len(samples)-2]
	rate := math.Abs(tail-prev) / span
	order := fracDiffOrder(rate, span)
	weights, width := buildFracDiffWeights(
		order,
		span,
		prev,
		nil,
		fracDiffLagLimit(span, defaultFracDiffMaxLag),
	)
	value := 0.0

	for lag := 0; lag < width && lag < len(samples); lag++ {
		value += weights[lag] * samples[len(samples)-1-lag]
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

func fracDiffWeightThreshold(span float64, reference float64) float64 {
	if span > 0 {
		return 1 / span
	}

	if reference > 0 {
		return 1 / reference
	}

	return 1
}

func fracDiffOrder(rate float64, span float64) float64 {
	if rate <= 0 {
		return 1 / (span + 1)
	}

	if rate >= 1 {
		return 1 - 1/(span+1)
	}

	return rate
}

func buildFracDiffWeights(
	order float64,
	span float64,
	reference float64,
	scratch []float64,
	maxLag int,
) ([]float64, int) {
	threshold := fracDiffWeightThreshold(span, reference)
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

func fracDiffLagLimit(span float64, maxLag int) int {
	lag := fracDiffMaxLag(span)

	if maxLag <= 0 {
		maxLag = defaultFracDiffMaxLag
	}

	if lag > maxLag {
		return maxLag
	}

	return lag
}

func fracDiffMaxLag(span float64) int {
	if span < 1 {
		return 1
	}

	return int(span) + 1
}

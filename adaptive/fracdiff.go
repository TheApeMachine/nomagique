package adaptive

import (
	"math"

	"github.com/theapemachine/nomagique/core"
)

/*
FracDiff applies a fixed-width fractional differencing filter to recent samples.
*/
type FracDiff[T ~float64] struct {
	Prev    float64
	Min     float64
	Max     float64
	Order   float64
	Ready   bool
	Width   int
	Head    int
	Count   int
	History []float64
	Weights []float64
	output  core.Scalar[T]
}

/*
NewFracDiff returns a fractional differencing stage ready to bootstrap from its first observation.
*/
func NewFracDiff[T ~float64](initial ...core.Number[T]) *FracDiff[T] {
	fractional := &FracDiff[T]{}

	if len(initial) > 0 {
		fractional.output = core.Scalar[T](0).Observe(initial...)
	}

	return fractional
}

/*
Observe absorbs the carried sample and returns the filtered value.
*/
func (fractional *FracDiff[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if len(inputs) == 0 {
		return fractional.output
	}

	sample, ok := inputs[0].(core.Scalar[T])

	if !ok {
		return fractional.output
	}

	if len(inputs) > 1 {
		if work, workOK := inputs[1].(core.Scalar[T]); workOK {
			sample = core.Scalar[T](T(sample) + T(work))
		}
	}

	fractional.output = core.Scalar[T](T(fractional.observe(float64(sample))))

	return fractional.output
}

/*
ObserveSample ingests one raw sample through the fractional differencing kernel.
*/
func (fractional *FracDiff[T]) ObserveSample(sample T) T {
	derived := T(fractional.observe(float64(sample)))
	fractional.output = core.Scalar[T](derived)

	return derived
}

/*
ObserveSamples writes one derived value per sample into out.
*/
func (fractional *FracDiff[T]) ObserveSamples(samples []T, out []T) {
	for index, sample := range samples {
		out[index] = fractional.ObserveSample(sample)
	}
}

/*
Reset clears derived state so the next Observe bootstraps again.
*/
func (fractional *FracDiff[T]) Reset() error {
	fractional.Prev = 0
	fractional.Min = 0
	fractional.Max = 0
	fractional.Order = 0
	fractional.Ready = false
	fractional.Width = 0
	fractional.Head = 0
	fractional.Count = 0
	fractional.History = nil
	fractional.Weights = nil
	fractional.output = core.Scalar[T](0)

	return nil
}

func (fractional *FracDiff[T]) observe(sample float64) float64 {
	if !fractional.Ready {
		fractional.Min = sample
		fractional.Max = sample
		fractional.Prev = sample
		fractional.Order = 0
		fractional.Ready = true
		fractional.Width = 1
		fractional.Head = 0
		fractional.Count = 1
		fractional.History = make([]float64, fracDiffMaxLag(0)+1)
		fractional.History[0] = sample
		fractional.Weights = []float64{1}

		return sample
	}

	return fractional.observeReady(sample)
}

func (fractional *FracDiff[T]) observeReady(sample float64) float64 {
	fractional.Min = math.Min(fractional.Min, sample)
	fractional.Max = math.Max(fractional.Max, sample)

	span := fractional.Max - fractional.Min

	if span == 0 {
		fractional.pushHistory(sample)
		fractional.Prev = sample

		return sample
	}

	rate := math.Abs(sample-fractional.Prev) / span
	order := fracDiffOrder(rate, span)
	fractional.maybeRebuildWeights(order, span)
	fractional.pushHistory(sample)
	fractional.Prev = sample

	return fractional.outputSum()
}

func (fractional *FracDiff[T]) maybeRebuildWeights(order float64, span float64) {
	if order == fractional.Order && fractional.Width > 0 {
		return
	}

	fractional.Order = order

	capacity := fracDiffMaxLag(span) + 1

	if cap(fractional.Weights) < capacity {
		fractional.Weights = make([]float64, 0, capacity)
	}

	weights, width := buildFracDiffWeights(order, span, fractional.Prev, fractional.Weights[:0])
	fractional.Weights = weights[:width]
	fractional.Width = width
	fractional.ensureHistoryCapacity(capacity)
}

func (fractional *FracDiff[T]) ensureHistoryCapacity(capacity int) {
	if len(fractional.History) >= capacity {
		return
	}

	next := make([]float64, capacity)
	copy(next, fractional.History)

	if fractional.Count > 0 {
		for index := 0; index < fractional.Count; index++ {
			source := (fractional.Head - index + len(fractional.History)) % len(fractional.History)
			next[index] = fractional.History[source]
		}

		fractional.Head = fractional.Count - 1
	}

	fractional.History = next
}

func (fractional *FracDiff[T]) pushHistory(sample float64) {
	if len(fractional.History) == 0 {
		return
	}

	fractional.Head = (fractional.Head + 1) % len(fractional.History)
	fractional.History[fractional.Head] = sample

	if fractional.Count < len(fractional.History) {
		fractional.Count++
	}
}

func (fractional *FracDiff[T]) outputSum() float64 {
	sum := 0.0
	limit := fractional.Width

	if fractional.Count < limit {
		limit = fractional.Count
	}

	for lag := 0; lag < limit; lag++ {
		index := fractional.Head - lag

		if index < 0 {
			index += len(fractional.History)
		}

		sum += fractional.Weights[lag] * fractional.History[index]
	}

	return sum
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
	order float64, span float64, reference float64, scratch []float64,
) ([]float64, int) {
	threshold := fracDiffWeightThreshold(span, reference)
	maxLag := fracDiffMaxLag(span)
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

func fracDiffMaxLag(span float64) int {
	if span < 1 {
		return 1
	}

	return int(span) + 1
}

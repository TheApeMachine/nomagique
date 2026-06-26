package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
FracDiff applies a fractional differencing filter to recent samples.
The constructor artifact holds config; Write buffers inbound payload.
*/
type FracDiff struct {
	artifact *datura.Artifact
	history  []float64
	weights  []float64
	min      float64
	max      float64
	prev     float64
	order    float64
	width    int
	head     int
	count    int
	ready    bool
}

/*
NewFracDiff returns a fractional differencing stage wired from config attributes on the artifact.
*/
func NewFracDiff(artifact *datura.Artifact) *FracDiff {
	return &FracDiff{
		artifact: artifact,
	}
}

func (fractional *FracDiff) Read(payload []byte) (int, error) {
	state := datura.Acquire("fracdiff-state", datura.APPJSON)

	if _, err := state.Unpack(fractional.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"fracdiff: state write failed",
			err,
		))
	}

	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"fracdiff: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"fracdiff: inputs required",
			nil,
		))
	}

	for index, input := range inputs {
		var sample float64

		if rootKey == "features" {
			features := datura.Peek[[]float64](state, rootKey)

			if index >= len(features) {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"fracdiff: feature index out of range",
					nil,
				))
			}

			sample = features[index]
		}

		if rootKey != "features" {
			sample = datura.Peek[float64](state, rootKey, input)
		}

		if math.IsNaN(sample) || math.IsInf(sample, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"fracdiff: sample is non-finite",
				nil,
			))
		}

		if !fractional.ready {
			capacity := fracDiffMaxLag(0) + 1
			fractional.history = make([]float64, capacity)
			fractional.history[0] = sample
			fractional.min = sample
			fractional.max = sample
			fractional.prev = sample
			fractional.order = 0
			fractional.width = 1
			fractional.head = 0
			fractional.count = 1
			fractional.weights = []float64{1}
			fractional.ready = true
			state.MergeOutput("value", sample)

			break
		}

		fractional.min = math.Min(fractional.min, sample)
		fractional.max = math.Max(fractional.max, sample)

		span := fractional.max - fractional.min

		if span == 0 {
			fractional.pushHistory(sample)

			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"fracdiff: sample span is zero",
				nil,
			))
		}

		rate := math.Abs(sample-fractional.prev) / span
		order := fracDiffOrder(rate, span)
		fractional.rebuildWeights(order, span)
		fractional.pushHistory(sample)
		fractional.prev = sample
		value := fractional.outputSum()

		state.MergeOutput("value", value)
	}

	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")

	return state.PackInto(payload)
}

func (fractional *FracDiff) Write(p []byte) (int, error) {
	fractional.artifact.WithPayload(p)
	return len(p), nil
}

func (fractional *FracDiff) Close() error {
	return nil
}

func (fractional *FracDiff) rebuildWeights(order float64, span float64) {
	if order == fractional.order && fractional.width > 0 {
		return
	}

	fractional.order = order

	capacity := fracDiffMaxLag(span) + 1
	weights := make([]float64, 0, capacity)
	weights, width := buildFracDiffWeights(order, span, fractional.prev, weights)
	fractional.width = width

	if len(fractional.history) < capacity {
		next := make([]float64, capacity)
		copy(next, fractional.history)

		if fractional.count > 0 {
			for index := 0; index < fractional.count; index++ {
				source := (fractional.head - index + len(fractional.history)) % len(fractional.history)
				next[index] = fractional.history[source]
			}

			fractional.head = fractional.count - 1
		}

		fractional.history = next
	}

	fractional.weights = weights[:width]
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

package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/statistic"
)

/*
FracDiff applies a fractional differencing filter to recent samples.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type FracDiff struct {
	artifact *datura.Artifact
}

/*
NewFracDiff returns a fractional differencing stage wired from config attributes on the artifact.
*/
func NewFracDiff(artifact *datura.Artifact) *FracDiff {
	artifact.Inspect("adaptive", "fracdiff", "NewFracDiff()")

	return &FracDiff{
		artifact: artifact,
	}
}

func (fractional *FracDiff) Write(payload []byte) (int, error) {
	fractional.artifact.WithPayload(payload)
	return len(payload), nil
}

func (fractional *FracDiff) Read(payload []byte) (int, error) {
	state := datura.Acquire("fracdiff-state", datura.APPJSON)
	state.Inspect("adaptive", "fracdiff", "Read()", "p")

	if _, err := state.Write(fractional.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	features := statistic.SnapshotFeatures(state)
	sampleKey := statistic.WireInputKey(fractional.artifact, state, "sample")
	sample, err := statistic.WireScalar(fractional.artifact, state, sampleKey)

	if err != nil {
		return 0, err
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"fracdiff: sample is non-finite",
			nil,
		))
	}

	output := datura.Peek[datura.Map[float64]](fractional.artifact, "output")

	if output == nil {
		capacity := float64(fracDiffMaxLag(0) + 1)
		history := make([]float64, int(capacity))
		history[0] = sample

		output = datura.Map[float64]{
			"min":   sample,
			"max":   sample,
			"prev":  sample,
			"order": 0,
			"width": 1,
			"head":  0,
			"count": 1,
			"value": sample,
		}

		fractional.artifact.Poke(output, "output")
		fractional.artifact.Poke(history, "history")
		fractional.artifact.Poke([]float64{1}, "weights")
		state.MergeOutput("value", output["value"])
		features.Restore(state)
		state.Merge("root", "output")

		if len(datura.Peek[[]string](state, "inputs")) == 0 {
			state.Merge("inputs", []string{"value"})
		}

		return state.Read(payload)
	}

	output["min"] = math.Min(output["min"], sample)
	output["max"] = math.Max(output["max"], sample)

	span := output["max"] - output["min"]

	if span == 0 {
		fractional.pushHistory(sample, output)
		output["prev"] = sample
		fractional.artifact.Poke(output, "output")

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"fracdiff: sample span is zero",
			nil,
		))
	}

	rate := math.Abs(sample-output["prev"]) / span
	order := fracDiffOrder(rate, span)
	fractional.rebuildWeights(order, span, output)
	fractional.pushHistory(sample, output)
	output["prev"] = sample
	output["value"] = fractional.outputSum(output)

	fractional.artifact.Poke(output, "output")
	state.MergeOutput("value", output["value"])
	features.Restore(state)
	state.Merge("root", "output")

	if len(datura.Peek[[]string](state, "inputs")) == 0 {
		state.Merge("inputs", []string{"value"})
	}

	return state.Read(payload)
}

func (fractional *FracDiff) Close() error {
	return nil
}

func (fractional *FracDiff) rebuildWeights(
	order float64,
	span float64,
	output datura.Map[float64],
) {
	if order == output["order"] && output["width"] > 0 {
		return
	}

	output["order"] = order

	capacity := fracDiffMaxLag(span) + 1
	weights := make([]float64, 0, capacity)
	weights, width := buildFracDiffWeights(order, span, output["prev"], weights)
	output["width"] = float64(width)

	fractional.ensureHistoryCapacity(int(capacity), output)
	fractional.artifact.Poke(weights[:width], "weights")
}

func (fractional *FracDiff) ensureHistoryCapacity(
	capacity int,
	output datura.Map[float64],
) {
	history := datura.Peek[[]float64](fractional.artifact, "history")

	if len(history) >= capacity {
		return
	}

	next := make([]float64, capacity)
	copy(next, history)

	head := int(output["head"])
	count := int(output["count"])

	if count > 0 {
		for index := 0; index < count; index++ {
			source := (head - index + len(history)) % len(history)
			next[index] = history[source]
		}

		output["head"] = float64(count - 1)
	}

	fractional.artifact.Poke(next, "history")
}

func (fractional *FracDiff) pushHistory(
	sample float64,
	output datura.Map[float64],
) {
	history := datura.Peek[[]float64](fractional.artifact, "history")

	if len(history) == 0 {
		return
	}

	head := int(output["head"])
	head = (head + 1) % len(history)
	history[head] = sample
	output["head"] = float64(head)

	count := int(output["count"])

	if count < len(history) {
		output["count"] = float64(count + 1)
	}

	fractional.artifact.Poke(history, "history")
}

func (fractional *FracDiff) outputSum(output datura.Map[float64]) float64 {
	history := datura.Peek[[]float64](fractional.artifact, "history")
	weights := datura.Peek[[]float64](fractional.artifact, "weights")

	sum := 0.0
	limit := int(output["width"])
	count := int(output["count"])

	if count < limit {
		limit = count
	}

	head := int(output["head"])

	for lag := 0; lag < limit; lag++ {
		index := head - lag

		if index < 0 {
			index += len(history)
		}

		sum += weights[lag] * history[index]
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

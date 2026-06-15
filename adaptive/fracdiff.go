package adaptive

import (
	"encoding/binary"
	"math"

	"github.com/theapemachine/datura"
)

/*
FracDiff applies a fixed-width fractional differencing filter to recent samples.
*/
type FracDiff struct {
	artifact *datura.Artifact
	Prev     float64
	Min      float64
	Max      float64
	Order    float64
	Ready    bool
	Width    int
	Head     int
	Count    int
	History  []float64
	Weights  []float64
}

/*
NewFracDiff returns a fractional differencing stage ready to bootstrap from its first observation.
*/
func NewFracDiff() *FracDiff {
	return &FracDiff{
		artifact: datura.Acquire("fracdiff", datura.Artifact_Type_json),
	}
}

func (fractional *FracDiff) Write(p []byte) (int, error) {
	return fractional.artifact.Write(p)
}

func (fractional *FracDiff) Read(p []byte) (int, error) {
	payload, err := fractional.artifact.Payload()

	if err == nil && len(payload) == 8 {
		sample := math.Float64frombits(binary.BigEndian.Uint64(payload))
		derived := fractional.step(sample)
		assignScalarPayload(&fractional.artifact, "frac-diff", derived)
	}

	return fractional.artifact.Read(p)
}

func (fractional *FracDiff) Close() error {
	return nil
}

/*
ObserveSample ingests one raw sample through the fractional differencing kernel.
*/
func (fractional *FracDiff) ObserveSample(sample float64) float64 {
	return fractional.readSampleViaArtifact(sample)
}

/*
ObserveSamples writes one derived value per sample into out.
*/
func (fractional *FracDiff) ObserveSamples(samples []float64, out []float64) {
	for index, sample := range samples {
		out[index] = fractional.ObserveSample(sample)
	}
}

/*
Reset clears derived state so the next Read bootstraps again.
*/
func (fractional *FracDiff) Reset() error {
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

	return nil
}

func (fractional *FracDiff) readSampleViaArtifact(sample float64) float64 {
	inbound := datura.Acquire("fracdiff-in", datura.Artifact_Type_json)
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, math.Float64bits(sample))
	_ = inbound.SetPayload(payload)
	buf, _ := inbound.Message().Marshal()
	_, _ = fractional.Write(buf)
	outBuf := make([]byte, 4096)
	_, _ = fractional.Read(outBuf)
	outbound := datura.Acquire("stage-out", datura.Artifact_Type_json)
	_, _ = outbound.Write(outBuf)
	readPayload, _ := outbound.Payload()

	if len(readPayload) != 8 {
		return 0
	}

	return math.Float64frombits(binary.BigEndian.Uint64(readPayload))
}

func (fractional *FracDiff) step(sample float64) float64 {
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

	return fractional.stepReady(sample)
}

func (fractional *FracDiff) stepReady(sample float64) float64 {
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

func (fractional *FracDiff) maybeRebuildWeights(order float64, span float64) {
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

func (fractional *FracDiff) ensureHistoryCapacity(capacity int) {
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

func (fractional *FracDiff) pushHistory(sample float64) {
	if len(fractional.History) == 0 {
		return
	}

	fractional.Head = (fractional.Head + 1) % len(fractional.History)
	fractional.History[fractional.Head] = sample

	if fractional.Count < len(fractional.History) {
		fractional.Count++
	}
}

func (fractional *FracDiff) outputSum() float64 {
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

/*
Value returns the last derived scalar without re-processing the stage.
*/
func (fractional *FracDiff) Value() float64 {
	return valueFromArtifact(fractional.artifact)
}

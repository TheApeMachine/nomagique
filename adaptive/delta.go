package adaptive

import (
	"encoding/binary"
	"math"

	"github.com/theapemachine/datura"
)

/*
Delta tracks a unit-normalized change relative to the running sample range.
*/
type Delta struct {
	artifact *datura.Artifact
	Prev     float64
	Min      float64
	Max      float64
	Ready    bool
}

/*
NewDelta returns a delta stage ready to bootstrap from its first observation.
*/
func NewDelta() *Delta {
	return &Delta{
		artifact: datura.Acquire("delta", datura.Artifact_Type_json),
	}
}

func (delta *Delta) Write(p []byte) (int, error) {
	return delta.artifact.Write(p)
}

func (delta *Delta) Read(p []byte) (int, error) {
	payload, err := delta.artifact.Payload()

	if err == nil && len(payload) == 8 {
		sample := math.Float64frombits(binary.BigEndian.Uint64(payload))
		derived := delta.step(sample)
		assignScalarPayload(&delta.artifact, "delta", derived)
	}

	return delta.artifact.Read(p)
}

func (delta *Delta) Close() error {
	return nil
}

/*
ObserveSample ingests one raw sample through the delta kernel.
*/
func (delta *Delta) ObserveSample(sample float64) float64 {
	return delta.readSampleViaArtifact(sample)
}

/*
ObserveSamples writes one derived value per sample into out.
*/
func (delta *Delta) ObserveSamples(samples []float64, out []float64) {
	for index, sample := range samples {
		out[index] = delta.ObserveSample(sample)
	}
}

/*
Reset clears derived state so the next Read bootstraps again.
*/
func (delta *Delta) Reset() error {
	delta.Prev = 0
	delta.Min = 0
	delta.Max = 0
	delta.Ready = false

	return nil
}

func (delta *Delta) readSampleViaArtifact(sample float64) float64 {
	inbound := datura.Acquire("delta-in", datura.Artifact_Type_json)
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, math.Float64bits(sample))
	_ = inbound.SetPayload(payload)
	buf, _ := inbound.Message().Marshal()
	_, _ = delta.Write(buf)
	outBuf := make([]byte, 4096)
	_, _ = delta.Read(outBuf)
	outbound := datura.Acquire("stage-out", datura.Artifact_Type_json)
	_, _ = outbound.Write(outBuf)
	readPayload, _ := outbound.Payload()

	if len(readPayload) != 8 {
		return 0
	}

	return math.Float64frombits(binary.BigEndian.Uint64(readPayload))
}

func (delta *Delta) step(sample float64) float64 {
	if !delta.Ready {
		delta.Prev = sample
		delta.Min = sample
		delta.Max = sample
		delta.Ready = true

		return 0
	}

	return delta.stepReady(sample)
}

func (delta *Delta) stepReady(sample float64) float64 {
	delta.Min = math.Min(delta.Min, sample)
	delta.Max = math.Max(delta.Max, sample)

	span := delta.Max - delta.Min

	if span == 0 {
		delta.Prev = sample

		return 0
	}

	normalized := math.Abs(sample-delta.Prev) / span
	delta.Prev = sample

	return normalized
}

/*
Value returns the last derived scalar without re-processing the stage.
*/
func (delta *Delta) Value() float64 {
	return valueFromArtifact(delta.artifact)
}

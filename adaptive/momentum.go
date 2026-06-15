package adaptive

import (
	"encoding/binary"
	"math"

	"github.com/theapemachine/datura"
)

/*
Momentum tracks a signed unit-normalized move relative to the running range.
*/
type Momentum struct {
	artifact *datura.Artifact
	Prev     float64
	Min      float64
	Max      float64
	Ready    bool
}

/*
NewMomentum returns a momentum stage ready to bootstrap from its first observation.
*/
func NewMomentum() *Momentum {
	return &Momentum{
		artifact: datura.Acquire("momentum", datura.Artifact_Type_json),
	}
}

func (momentum *Momentum) Write(p []byte) (int, error) {
	return momentum.artifact.Write(p)
}

func (momentum *Momentum) Read(p []byte) (int, error) {
	payload, err := momentum.artifact.Payload()

	if err == nil && len(payload) == 8 {
		sample := math.Float64frombits(binary.BigEndian.Uint64(payload))
		derived := momentum.step(sample)
		assignScalarPayload(&momentum.artifact, "momentum", derived)
	}

	return momentum.artifact.Read(p)
}

func (momentum *Momentum) Close() error {
	return nil
}

/*
ObserveSample ingests one raw sample through the momentum kernel.
*/
func (momentum *Momentum) ObserveSample(sample float64) float64 {
	return momentum.readSampleViaArtifact(sample)
}

/*
ObserveSamples writes one derived value per sample into out.
*/
func (momentum *Momentum) ObserveSamples(samples []float64, out []float64) {
	for index, sample := range samples {
		out[index] = momentum.ObserveSample(sample)
	}
}

/*
Reset clears derived state so the next Read bootstraps again.
*/
func (momentum *Momentum) Reset() error {
	momentum.Prev = 0
	momentum.Min = 0
	momentum.Max = 0
	momentum.Ready = false

	return nil
}

func (momentum *Momentum) readSampleViaArtifact(sample float64) float64 {
	inbound := datura.Acquire("momentum-in", datura.Artifact_Type_json)
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, math.Float64bits(sample))
	_ = inbound.SetPayload(payload)
	buf, _ := inbound.Message().Marshal()
	_, _ = momentum.Write(buf)
	outBuf := make([]byte, 4096)
	_, _ = momentum.Read(outBuf)
	outbound := datura.Acquire("stage-out", datura.Artifact_Type_json)
	_, _ = outbound.Write(outBuf)
	readPayload, _ := outbound.Payload()

	if len(readPayload) != 8 {
		return 0
	}

	return math.Float64frombits(binary.BigEndian.Uint64(readPayload))
}

func (momentum *Momentum) step(sample float64) float64 {
	if !momentum.Ready {
		momentum.Prev = sample
		momentum.Min = sample
		momentum.Max = sample
		momentum.Ready = true

		return 0
	}

	return momentum.stepReady(sample)
}

func (momentum *Momentum) stepReady(sample float64) float64 {
	momentum.Min = math.Min(momentum.Min, sample)
	momentum.Max = math.Max(momentum.Max, sample)

	span := momentum.Max - momentum.Min

	if span == 0 {
		momentum.Prev = sample

		return 0
	}

	signed := (sample - momentum.Prev) / span
	momentum.Prev = sample

	return signed
}

/*
Value returns the last derived scalar without re-processing the stage.
*/
func (momentum *Momentum) Value() float64 {
	return valueFromArtifact(momentum.artifact)
}

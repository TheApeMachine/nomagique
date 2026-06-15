package adaptive

import (
	"encoding/binary"
	"math"

	"github.com/theapemachine/datura"
)

/*
Compression scores how far below the running baseline the current sample sits.
*/
type Compression struct {
	artifact *datura.Artifact
	Baseline float64
	Ready    bool
}

/*
NewCompression returns a compression stage ready to bootstrap from its first observation.
*/
func NewCompression() *Compression {
	return &Compression{
		artifact: datura.Acquire("compression", datura.Artifact_Type_json),
	}
}

func (compression *Compression) Write(p []byte) (int, error) {
	return compression.artifact.Write(p)
}

func (compression *Compression) Read(p []byte) (int, error) {
	payload, err := compression.artifact.Payload()

	if err == nil && len(payload) == 8 {
		sample := math.Float64frombits(binary.BigEndian.Uint64(payload))
		derived := compression.step(sample)
		assignScalarPayload(&compression.artifact, "compression", derived)
	}

	return compression.artifact.Read(p)
}

func (compression *Compression) Close() error {
	return nil
}

/*
ObserveSample ingests one raw sample through the compression kernel.
*/
func (compression *Compression) ObserveSample(sample float64) float64 {
	return compression.readSampleViaArtifact(sample)
}

/*
ObserveSamples writes one derived value per sample into out.
*/
func (compression *Compression) ObserveSamples(samples []float64, out []float64) {
	for index, sample := range samples {
		out[index] = compression.ObserveSample(sample)
	}
}

/*
Reset clears derived state so the next Read bootstraps again.
*/
func (compression *Compression) Reset() error {
	compression.Baseline = 0
	compression.Ready = false

	return nil
}

func (compression *Compression) readSampleViaArtifact(sample float64) float64 {
	inbound := datura.Acquire("compression-in", datura.Artifact_Type_json)
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, math.Float64bits(sample))
	_ = inbound.SetPayload(payload)
	buf, _ := inbound.Message().Marshal()
	_, _ = compression.Write(buf)
	outBuf := make([]byte, 4096)
	_, _ = compression.Read(outBuf)
	outbound := datura.Acquire("stage-out", datura.Artifact_Type_json)
	_, _ = outbound.Write(outBuf)
	readPayload, _ := outbound.Payload()

	if len(readPayload) != 8 {
		return 0
	}

	return math.Float64frombits(binary.BigEndian.Uint64(readPayload))
}

func (compression *Compression) step(sample float64) float64 {
	if !compression.Ready {
		compression.Baseline = sample
		compression.Ready = true

		return 0
	}

	return compression.stepReady(sample)
}

func (compression *Compression) stepReady(sample float64) float64 {
	if sample > compression.Baseline {
		compression.Baseline = sample

		return 0
	}

	if compression.Baseline == 0 {
		return 0
	}

	return (compression.Baseline - sample) / compression.Baseline
}

/*
Value returns the last derived scalar without re-processing the stage.
*/
func (compression *Compression) Value() float64 {
	return valueFromArtifact(compression.artifact)
}

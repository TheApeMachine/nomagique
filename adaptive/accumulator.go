package adaptive

import (
	"encoding/binary"
	"math"

	"github.com/theapemachine/datura"
)

/*
Accumulator integrates signed signal strength into a level with no bounds.
*/
type Accumulator struct {
	artifact *datura.Artifact
	Level    float64
}

/*
NewAccumulator returns an accumulator stage ready for its first observation.
*/
func NewAccumulator() *Accumulator {
	return &Accumulator{
		artifact: datura.Acquire("accumulator", datura.Artifact_Type_json),
	}
}

func (accumulator *Accumulator) Write(p []byte) (int, error) {
	return accumulator.artifact.Write(p)
}

func (accumulator *Accumulator) Read(p []byte) (int, error) {
	payload, err := accumulator.artifact.Payload()

	if err == nil && len(payload) == 8 {
		sample := math.Float64frombits(binary.BigEndian.Uint64(payload))
		derived := accumulator.step(sample)
		assignScalarPayload(&accumulator.artifact, "accumulator", derived)
	}

	return accumulator.artifact.Read(p)
}

func (accumulator *Accumulator) Close() error {
	return nil
}

/*
ObserveSample ingests one raw sample through the accumulator kernel.
*/
func (accumulator *Accumulator) ObserveSample(sample float64) float64 {
	return accumulator.readSampleViaArtifact(sample)
}

/*
ObserveSamples writes one derived value per sample into out.
*/
func (accumulator *Accumulator) ObserveSamples(samples []float64, out []float64) {
	for index, sample := range samples {
		out[index] = accumulator.ObserveSample(sample)
	}
}

/*
Reset clears derived state.
*/
func (accumulator *Accumulator) Reset() error {
	accumulator.Level = 0

	return nil
}

func (accumulator *Accumulator) readSampleViaArtifact(sample float64) float64 {
	inbound := datura.Acquire("accumulator-in", datura.Artifact_Type_json)
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, math.Float64bits(sample))
	_ = inbound.SetPayload(payload)
	buf, _ := inbound.Message().Marshal()
	_, _ = accumulator.Write(buf)
	outBuf := make([]byte, 4096)
	_, _ = accumulator.Read(outBuf)
	outbound := datura.Acquire("stage-out", datura.Artifact_Type_json)
	_, _ = outbound.Write(outBuf)
	readPayload, _ := outbound.Payload()

	if len(readPayload) != 8 {
		return 0
	}

	return math.Float64frombits(binary.BigEndian.Uint64(readPayload))
}

func (accumulator *Accumulator) step(sample float64) float64 {
	if sample == 0 {
		return accumulator.Level
	}

	accumulator.Level += sample

	return accumulator.Level
}

/*
Value returns the last derived scalar without re-processing the stage.
*/
func (accumulator *Accumulator) Value() float64 {
	return valueFromArtifact(accumulator.artifact)
}

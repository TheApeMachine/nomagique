package adaptive

import (
	"encoding/binary"
	"math"

	"github.com/theapemachine/datura"
)

/*
Range tracks the running span of observed samples.
*/
type Range struct {
	artifact *datura.Artifact
	Min      float64
	Max      float64
	Ready    bool
}

/*
NewRange returns a range stage ready to bootstrap from its first observation.
*/
func NewRange() *Range {
	return &Range{
		artifact: datura.Acquire("range", datura.Artifact_Type_json),
	}
}

func (extent *Range) Write(p []byte) (int, error) {
	return extent.artifact.Write(p)
}

func (extent *Range) Read(p []byte) (int, error) {
	payload, err := extent.artifact.Payload()

	if err == nil && len(payload) == 8 {
		sample := math.Float64frombits(binary.BigEndian.Uint64(payload))
		derived := extent.step(sample)
		assignScalarPayload(&extent.artifact, "range", derived)
	}

	return extent.artifact.Read(p)
}

func (extent *Range) Close() error {
	return nil
}

/*
ObserveSample ingests one raw sample through the range kernel.
*/
func (extent *Range) ObserveSample(sample float64) float64 {
	return extent.readSampleViaArtifact(sample)
}

/*
ObserveSamples writes one derived value per sample into out.
*/
func (extent *Range) ObserveSamples(samples []float64, out []float64) {
	for index, sample := range samples {
		out[index] = extent.ObserveSample(sample)
	}
}

/*
Reset clears derived state so the next Read bootstraps again.
*/
func (extent *Range) Reset() error {
	extent.Min = 0
	extent.Max = 0
	extent.Ready = false

	return nil
}

func (extent *Range) readSampleViaArtifact(sample float64) float64 {
	inbound := datura.Acquire("range-in", datura.Artifact_Type_json)
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, math.Float64bits(sample))
	_ = inbound.SetPayload(payload)
	buf, _ := inbound.Message().Marshal()
	_, _ = extent.Write(buf)
	outBuf := make([]byte, 4096)
	_, _ = extent.Read(outBuf)
	outbound := datura.Acquire("stage-out", datura.Artifact_Type_json)
	_, _ = outbound.Write(outBuf)
	readPayload, _ := outbound.Payload()

	if len(readPayload) != 8 {
		return 0
	}

	return math.Float64frombits(binary.BigEndian.Uint64(readPayload))
}

func (extent *Range) step(sample float64) float64 {
	if !extent.Ready {
		extent.Min = sample
		extent.Max = sample
		extent.Ready = true

		return 0
	}

	return extent.stepReady(sample)
}

func (extent *Range) stepReady(sample float64) float64 {
	extent.Min = math.Min(extent.Min, sample)
	extent.Max = math.Max(extent.Max, sample)

	return extent.Max - extent.Min
}

/*
Value returns the last derived scalar without re-processing the stage.
*/
func (extent *Range) Value() float64 {
	return valueFromArtifact(extent.artifact)
}

package adaptive

import (
	"encoding/binary"
	"math"

	"github.com/theapemachine/datura"
)

/*
Variance tracks an adaptive mean and variance from the observed sample stream.
*/
type Variance struct {
	artifact *datura.Artifact
	Mean     float64
	Var      float64
	Prev     float64
	Min      float64
	Max      float64
	Rate     float64
	Ready    bool
}

/*
NewVariance returns a variance stage ready to bootstrap from its first observation.
*/
func NewVariance() *Variance {
	return &Variance{
		artifact: datura.Acquire("variance", datura.Artifact_Type_json),
	}
}

func (variance *Variance) Write(p []byte) (int, error) {
	return variance.artifact.Write(p)
}

func (variance *Variance) Read(p []byte) (int, error) {
	payload, err := variance.artifact.Payload()

	if err == nil && len(payload) == 8 {
		sample := math.Float64frombits(binary.BigEndian.Uint64(payload))
		derived := variance.step(sample)
		assignScalarPayload(&variance.artifact, "variance", derived)
	}

	return variance.artifact.Read(p)
}

func (variance *Variance) Close() error {
	return nil
}

/*
ObserveSample ingests one raw sample through the variance kernel.
*/
func (variance *Variance) ObserveSample(sample float64) float64 {
	return variance.readSampleViaArtifact(sample)
}

/*
ObserveSamples writes one derived value per sample into out.
*/
func (variance *Variance) ObserveSamples(samples []float64, out []float64) {
	for index, sample := range samples {
		out[index] = variance.ObserveSample(sample)
	}
}

/*
Reset clears derived state so the next Read bootstraps again.
*/
func (variance *Variance) Reset() error {
	variance.Mean = 0
	variance.Var = 0
	variance.Prev = 0
	variance.Min = 0
	variance.Max = 0
	variance.Rate = 0
	variance.Ready = false

	return nil
}

func (variance *Variance) readSampleViaArtifact(sample float64) float64 {
	inbound := datura.Acquire("variance-in", datura.Artifact_Type_json)
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, math.Float64bits(sample))
	_ = inbound.SetPayload(payload)
	buf, _ := inbound.Message().Marshal()
	_, _ = variance.Write(buf)
	outBuf := make([]byte, 4096)
	_, _ = variance.Read(outBuf)
	outbound := datura.Acquire("stage-out", datura.Artifact_Type_json)
	_, _ = outbound.Write(outBuf)
	readPayload, _ := outbound.Payload()

	if len(readPayload) != 8 {
		return 0
	}

	return math.Float64frombits(binary.BigEndian.Uint64(readPayload))
}

func (variance *Variance) step(sample float64) float64 {
	if !variance.Ready {
		variance.Mean = sample
		variance.Var = 0
		variance.Prev = sample
		variance.Min = sample
		variance.Max = sample
		variance.Ready = true

		return 0
	}

	return variance.stepReady(sample)
}

func (variance *Variance) stepReady(sample float64) float64 {
	variance.Min = math.Min(variance.Min, sample)
	variance.Max = math.Max(variance.Max, sample)

	span := variance.Max - variance.Min

	if span == 0 {
		variance.Prev = sample

		return variance.Var
	}

	delta := math.Abs(sample - variance.Prev)
	variance.Rate = delta / span
	deviation := sample - variance.Mean
	variance.Mean += variance.Rate * (sample - variance.Mean)
	variance.Var += variance.Rate * (deviation*deviation - variance.Var)
	variance.Prev = sample

	return variance.Var
}

/*
Value returns the last derived scalar without re-processing the stage.
*/
func (variance *Variance) Value() float64 {
	return valueFromArtifact(variance.artifact)
}

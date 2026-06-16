package adaptive

import (
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

	if err == nil {
		observeScalarPayload(&momentum.artifact, "momentum", payload, momentum.step)
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
	return observeScalarSample(&momentum.artifact, "momentum", sample, momentum.step)
}

/*
ObserveSamples writes one derived value per sample into out.
*/
func (momentum *Momentum) ObserveSamples(samples []float64, out []float64) {
	limit := len(samples)

	if len(out) < limit {
		limit = len(out)
	}

	for index := 0; index < limit; index++ {
		out[index] = momentum.ObserveSample(samples[index])
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

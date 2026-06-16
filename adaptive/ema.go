package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
EMA is a volatility-adaptive exponential moving average stage.
*/
type EMA struct {
	artifact *datura.Artifact
	value    float64
	prev     float64
	min      float64
	max      float64
	rate     float64
	ready    bool
}

/*
NewEMA returns an EMA stage ready to bootstrap from its first observation.
*/
func NewEMA() *EMA {
	return &EMA{
		artifact: datura.Acquire("ema", datura.Artifact_Type_json),
	}
}

func (ema *EMA) Write(p []byte) (int, error) {
	return ema.artifact.Write(p)
}

func (ema *EMA) Read(p []byte) (int, error) {
	payload, err := ema.artifact.Payload()

	if err == nil {
		observeScalarPayload(&ema.artifact, "ema", payload, ema.step)
	}

	return ema.artifact.Read(p)
}

func (ema *EMA) Close() error {
	return nil
}

/*
ObserveSample ingests one raw sample through the EMA kernel.
*/
func (ema *EMA) ObserveSample(sample float64) float64 {
	return observeScalarSample(&ema.artifact, "ema", sample, ema.step)
}

/*
ObserveSamples writes one derived value per sample into out.
*/
func (ema *EMA) ObserveSamples(samples []float64, out []float64) {
	limit := len(samples)

	if len(out) < limit {
		limit = len(out)
	}

	for index := 0; index < limit; index++ {
		out[index] = ema.ObserveSample(samples[index])
	}
}

/*
Reset clears derived state so the next observation bootstraps again.
*/
func (ema *EMA) Reset() error {
	ema.value = 0
	ema.prev = 0
	ema.min = 0
	ema.max = 0
	ema.rate = 0
	ema.ready = false

	return nil
}

func (ema *EMA) step(sample float64) float64 {
	if !ema.ready {
		ema.value = sample
		ema.prev = sample
		ema.min = sample
		ema.max = sample
		ema.ready = true

		return sample
	}

	ema.min = math.Min(ema.min, sample)
	ema.max = math.Max(ema.max, sample)

	span := ema.max - ema.min

	if span == 0 {
		ema.prev = sample

		return ema.value
	}

	delta := math.Abs(sample - ema.prev)
	ema.rate = delta / span
	ema.value += ema.rate * (sample - ema.value)
	ema.prev = sample

	return ema.value
}

/*
Value returns the last derived scalar without re-processing the stage.
*/
func (ema *EMA) Value() float64 {
	return valueFromArtifact(ema.artifact)
}

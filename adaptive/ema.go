package adaptive

import (
	"encoding/binary"
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

	if err == nil && len(payload) == 8 {
		sample := math.Float64frombits(binary.BigEndian.Uint64(payload))
		derived := ema.step(sample)
		assignScalarPayload(&ema.artifact, "ema", derived)
	}

	return ema.artifact.Read(p)
}

func (ema *EMA) Close() error {
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

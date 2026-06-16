package adaptive

import (
	"encoding/binary"
	"math"

	"github.com/theapemachine/datura"
)

/*
ZScore tracks adaptive scale for a normalized surprise score.
*/
type ZScore struct {
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
NewZScore returns a z-score stage ready to bootstrap from its first observation.
*/
func NewZScore() *ZScore {
	return &ZScore{
		artifact: datura.Acquire("zscore", datura.Artifact_Type_json),
	}
}

func (surprise *ZScore) Write(p []byte) (int, error) {
	return surprise.artifact.Write(p)
}

func (surprise *ZScore) Read(p []byte) (int, error) {
	payload, err := surprise.artifact.Payload()

	if err == nil && len(payload) >= 8 {
		sample := math.Float64frombits(binary.BigEndian.Uint64(payload[:8]))
		anchor := 0.0
		hasAnchor := false

		if len(payload) >= 16 {
			anchor = math.Float64frombits(binary.BigEndian.Uint64(payload[8:16]))
			hasAnchor = finiteScalar(anchor)
		}

		if finiteScalar(sample) {
			derived := surprise.step(sample, anchor, hasAnchor)

			if finiteScalar(derived) {
				assignScalarPayload(&surprise.artifact, "zscore", derived)
			}
		}
	}

	return surprise.artifact.Read(p)
}

func (surprise *ZScore) Close() error {
	return nil
}

/*
ObserveSample ingests one raw sample through the z-score kernel.
*/
func (surprise *ZScore) ObserveSample(sample float64) float64 {
	return observeScalarSample(&surprise.artifact, "zscore", sample, func(value float64) float64 {
		return surprise.step(value, 0, false)
	})
}

/*
ObserveSamples writes one derived value per sample into out.
*/
func (surprise *ZScore) ObserveSamples(samples []float64, out []float64) {
	limit := len(samples)

	if len(out) < limit {
		limit = len(out)
	}

	for index := 0; index < limit; index++ {
		out[index] = surprise.ObserveSample(samples[index])
	}
}

/*
Reset clears derived state so the next Read bootstraps again.
*/
func (surprise *ZScore) Reset() error {
	surprise.Mean = 0
	surprise.Var = 0
	surprise.Prev = 0
	surprise.Min = 0
	surprise.Max = 0
	surprise.Rate = 0
	surprise.Ready = false

	return nil
}

func (surprise *ZScore) step(sample float64, anchorMean float64, hasAnchorMean bool) float64 {
	if !surprise.Ready {
		surprise.Mean = sample
		surprise.Var = 0
		surprise.Prev = sample
		surprise.Min = sample
		surprise.Max = sample
		surprise.Ready = true

		return 0
	}

	return surprise.stepReady(sample, anchorMean, hasAnchorMean)
}

func (surprise *ZScore) stepReady(
	sample float64, anchorMean float64, hasAnchorMean bool,
) float64 {
	surprise.Min = math.Min(surprise.Min, sample)
	surprise.Max = math.Max(surprise.Max, sample)

	span := surprise.Max - surprise.Min

	if span == 0 {
		surprise.Prev = sample

		return 0
	}

	delta := math.Abs(sample - surprise.Prev)
	surprise.Rate = delta / span
	level := surprise.Mean

	if hasAnchorMean {
		level = anchorMean
	}

	deviation := sample - level

	if !hasAnchorMean {
		surprise.Mean += surprise.Rate * (sample - surprise.Mean)
	}

	surprise.Var += surprise.Rate * (deviation*deviation - surprise.Var)
	surprise.Prev = sample

	if surprise.Var <= 0 {
		return 0
	}

	return deviation / math.Sqrt(surprise.Var)
}

/*
Value returns the last derived scalar without re-processing the stage.
*/
func (surprise *ZScore) Value() float64 {
	return valueFromArtifact(surprise.artifact)
}

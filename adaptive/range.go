package adaptive

import (
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

	if err == nil {
		observeScalarPayload(&extent.artifact, "range", payload, extent.step)
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
	return observeScalarSample(&extent.artifact, "range", sample, extent.step)
}

/*
ObserveSamples writes one derived value per sample into out.
*/
func (extent *Range) ObserveSamples(samples []float64, out []float64) {
	limit := len(samples)

	if len(out) < limit {
		limit = len(out)
	}

	for index := 0; index < limit; index++ {
		out[index] = extent.ObserveSample(samples[index])
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

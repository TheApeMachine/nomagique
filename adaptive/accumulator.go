package adaptive

import "github.com/theapemachine/datura"

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

	if err == nil {
		observeScalarPayload(&accumulator.artifact, "accumulator", payload, accumulator.step)
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
	return observeScalarSample(&accumulator.artifact, "accumulator", sample, accumulator.step)
}

/*
ObserveSamples writes one derived value per sample into out.
*/
func (accumulator *Accumulator) ObserveSamples(samples []float64, out []float64) {
	limit := len(samples)

	if len(out) < limit {
		limit = len(out)
	}

	for index := 0; index < limit; index++ {
		out[index] = accumulator.ObserveSample(samples[index])
	}
}

/*
Reset clears derived state.
*/
func (accumulator *Accumulator) Reset() error {
	accumulator.Level = 0

	return nil
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

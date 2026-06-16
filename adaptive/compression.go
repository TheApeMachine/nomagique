package adaptive

import "github.com/theapemachine/datura"

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

	if err == nil {
		observeScalarPayload(&compression.artifact, "compression", payload, compression.step)
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
	return observeScalarSample(&compression.artifact, "compression", sample, compression.step)
}

/*
ObserveSamples writes one derived value per sample into out.
*/
func (compression *Compression) ObserveSamples(samples []float64, out []float64) {
	limit := len(samples)

	if len(out) < limit {
		limit = len(out)
	}

	for index := 0; index < limit; index++ {
		out[index] = compression.ObserveSample(samples[index])
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

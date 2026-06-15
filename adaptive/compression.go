package adaptive

import (
	"github.com/theapemachine/nomagique/core"
)

/*
Compression scores how far below the running baseline the current sample sits.
*/
type Compression[T ~float64] struct {
	Baseline float64
	Ready    bool
	output   core.Scalar[T]
}

/*
NewCompression returns a compression stage ready to bootstrap from its first observation.
*/
func NewCompression[T ~float64](initial ...core.Number[T]) *Compression[T] {
	compression := &Compression[T]{}

	if len(initial) > 0 {
		compression.output = core.Scalar[T](0).Observe(initial...)
	}

	return compression
}

/*
Observe absorbs the carried sample and returns the compression score.
*/
func (compression *Compression[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if len(inputs) == 0 {
		return compression.output
	}

	sample, ok := inputs[0].(core.Scalar[T])

	if !ok {
		return compression.output
	}

	if len(inputs) > 1 {
		if work, workOK := inputs[1].(core.Scalar[T]); workOK {
			sample = core.Scalar[T](T(sample) + T(work))
		}
	}

	compression.output = core.Scalar[T](T(compression.observe(float64(sample))))

	return compression.output
}

/*
ObserveSample ingests one raw sample through the compression kernel.
*/
func (compression *Compression[T]) ObserveSample(sample T) T {
	derived := T(compression.observe(float64(sample)))
	compression.output = core.Scalar[T](derived)

	return derived
}

/*
ObserveSamples writes one derived value per sample into out.
*/
func (compression *Compression[T]) ObserveSamples(samples []T, out []T) {
	for index, sample := range samples {
		out[index] = compression.ObserveSample(sample)
	}
}

/*
Reset clears derived state so the next Observe bootstraps again.
*/
func (compression *Compression[T]) Reset() error {
	compression.Baseline = 0
	compression.Ready = false
	compression.output = core.Scalar[T](0)

	return nil
}

func (compression *Compression[T]) observe(sample float64) float64 {
	if !compression.Ready {
		compression.Baseline = sample
		compression.Ready = true

		return 0
	}

	return compression.observeReady(sample)
}

func (compression *Compression[T]) observeReady(sample float64) float64 {
	if sample > compression.Baseline {
		compression.Baseline = sample

		return 0
	}

	if compression.Baseline == 0 {
		return 0
	}

	return (compression.Baseline - sample) / compression.Baseline
}

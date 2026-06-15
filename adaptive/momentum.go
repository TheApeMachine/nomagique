package adaptive

import (
	"math"

	"github.com/theapemachine/nomagique/core"
)

/*
Momentum tracks a signed unit-normalized move relative to the running range.
*/
type Momentum[T ~float64] struct {
	Prev   float64
	Min    float64
	Max    float64
	Ready  bool
	output core.Scalar[T]
}

/*
NewMomentum returns a momentum stage ready to bootstrap from its first observation.
*/
func NewMomentum[T ~float64](initial ...core.Number[T]) *Momentum[T] {
	momentum := &Momentum[T]{}

	if len(initial) > 0 {
		momentum.output = core.Scalar[T](0).Observe(initial...)
	}

	return momentum
}

/*
Observe absorbs the carried sample and returns signed normalized momentum.
*/
func (momentum *Momentum[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if len(inputs) == 0 {
		return momentum.output
	}

	sample, ok := inputs[0].(core.Scalar[T])

	if !ok {
		return momentum.output
	}

	if len(inputs) > 1 {
		if work, workOK := inputs[1].(core.Scalar[T]); workOK {
			sample = core.Scalar[T](T(sample) + T(work))
		}
	}

	momentum.output = core.Scalar[T](T(momentum.observe(float64(sample))))

	return momentum.output
}

/*
ObserveSample ingests one raw sample through the momentum kernel.
*/
func (momentum *Momentum[T]) ObserveSample(sample T) T {
	derived := T(momentum.observe(float64(sample)))
	momentum.output = core.Scalar[T](derived)

	return derived
}

/*
ObserveSamples writes one derived value per sample into out.
*/
func (momentum *Momentum[T]) ObserveSamples(samples []T, out []T) {
	for index, sample := range samples {
		out[index] = momentum.ObserveSample(sample)
	}
}

/*
Reset clears derived state so the next Observe bootstraps again.
*/
func (momentum *Momentum[T]) Reset() error {
	momentum.Prev = 0
	momentum.Min = 0
	momentum.Max = 0
	momentum.Ready = false
	momentum.output = core.Scalar[T](0)

	return nil
}

func (momentum *Momentum[T]) observe(sample float64) float64 {
	if !momentum.Ready {
		momentum.Prev = sample
		momentum.Min = sample
		momentum.Max = sample
		momentum.Ready = true

		return 0
	}

	return momentum.observeReady(sample)
}

func (momentum *Momentum[T]) observeReady(sample float64) float64 {
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

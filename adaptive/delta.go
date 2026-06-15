package adaptive

import (
	"math"

	"github.com/theapemachine/nomagique/core"
)

/*
Delta tracks a unit-normalized change relative to the running sample range.
*/
type Delta[T ~float64] struct {
	Prev   float64
	Min    float64
	Max    float64
	Ready  bool
	output core.Scalar[T]
}

/*
NewDelta returns a delta stage ready to bootstrap from its first observation.
*/
func NewDelta[T ~float64](initial ...core.Number[T]) *Delta[T] {
	delta := &Delta[T]{}

	if len(initial) > 0 {
		delta.output = core.Scalar[T](0).Observe(initial...)
	}

	return delta
}

/*
Observe absorbs the carried sample and derives the normalized delta.
*/
func (delta *Delta[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if len(inputs) == 0 {
		return delta.output
	}

	sample, ok := inputs[0].(core.Scalar[T])

	if !ok {
		return delta.output
	}

	if len(inputs) > 1 {
		if work, workOK := inputs[1].(core.Scalar[T]); workOK {
			sample = core.Scalar[T](T(sample) + T(work))
		}
	}

	delta.output = core.Scalar[T](T(delta.observe(float64(sample))))

	return delta.output
}

/*
ObserveSample ingests one raw sample through the delta kernel.
*/
func (delta *Delta[T]) ObserveSample(sample T) T {
	derived := T(delta.observe(float64(sample)))
	delta.output = core.Scalar[T](derived)

	return derived
}

/*
ObserveSamples writes one derived value per sample into out.
*/
func (delta *Delta[T]) ObserveSamples(samples []T, out []T) {
	for index, sample := range samples {
		out[index] = delta.ObserveSample(sample)
	}
}

/*
Reset clears derived state so the next Observe bootstraps again.
*/
func (delta *Delta[T]) Reset() error {
	delta.Prev = 0
	delta.Min = 0
	delta.Max = 0
	delta.Ready = false
	delta.output = core.Scalar[T](0)

	return nil
}

func (delta *Delta[T]) observe(sample float64) float64 {
	if !delta.Ready {
		delta.Prev = sample
		delta.Min = sample
		delta.Max = sample
		delta.Ready = true

		return 0
	}

	return delta.observeReady(sample)
}

func (delta *Delta[T]) observeReady(sample float64) float64 {
	delta.Min = math.Min(delta.Min, sample)
	delta.Max = math.Max(delta.Max, sample)

	span := delta.Max - delta.Min

	if span == 0 {
		delta.Prev = sample

		return 0
	}

	normalized := math.Abs(sample-delta.Prev) / span
	delta.Prev = sample

	return normalized
}

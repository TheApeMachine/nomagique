package adaptive

import (
	"github.com/theapemachine/nomagique/core"
)

/*
Accumulator integrates signed signal strength into a level with no bounds.
*/
type Accumulator[T ~float64] struct {
	Level  float64
	output core.Scalar[T]
}

/*
NewAccumulator returns an accumulator stage ready for its first observation.
*/
func NewAccumulator[T ~float64](initial ...core.Number[T]) *Accumulator[T] {
	accumulator := &Accumulator[T]{}

	if len(initial) > 0 {
		accumulator.output = core.Scalar[T](0).Observe(initial...)
	}

	return accumulator
}

/*
Observe absorbs the carried sample and integrates it into the level.
*/
func (accumulator *Accumulator[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if len(inputs) == 0 {
		return accumulator.output
	}

	sample, ok := inputs[0].(core.Scalar[T])

	if !ok {
		return accumulator.output
	}

	if len(inputs) > 1 {
		if work, workOK := inputs[1].(core.Scalar[T]); workOK {
			sample = core.Scalar[T](T(sample) + T(work))
		}
	}

	accumulator.output = core.Scalar[T](T(accumulator.observe(float64(sample))))

	return accumulator.output
}

/*
ObserveSample ingests one raw sample through the accumulator kernel.
*/
func (accumulator *Accumulator[T]) ObserveSample(sample T) T {
	derived := T(accumulator.observe(float64(sample)))
	accumulator.output = core.Scalar[T](derived)

	return derived
}

/*
ObserveSamples writes one derived value per sample into out.
*/
func (accumulator *Accumulator[T]) ObserveSamples(samples []T, out []T) {
	for index, sample := range samples {
		out[index] = accumulator.ObserveSample(sample)
	}
}

/*
Reset clears derived state.
*/
func (accumulator *Accumulator[T]) Reset() error {
	accumulator.Level = 0
	accumulator.output = core.Scalar[T](0)

	return nil
}

func (accumulator *Accumulator[T]) observe(sample float64) float64 {
	if sample == 0 {
		return accumulator.Level
	}

	accumulator.Level += sample

	return accumulator.Level
}

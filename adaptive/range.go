package adaptive

import (
	"math"

	"github.com/theapemachine/nomagique/core"
)

/*
Range tracks the running span of observed samples.
*/
type Range[T ~float64] struct {
	Min    float64
	Max    float64
	Ready  bool
	output core.Scalar[T]
}

/*
NewRange returns a range stage ready to bootstrap from its first observation.
*/
func NewRange[T ~float64](initial ...core.Number[T]) *Range[T] {
	extent := &Range[T]{}

	if len(initial) > 0 {
		extent.output = core.Scalar[T](0).Observe(initial...)
	}

	return extent
}

/*
Observe absorbs the carried sample and returns max minus min.
*/
func (extent *Range[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if len(inputs) == 0 {
		return extent.output
	}

	sample, ok := inputs[0].(core.Scalar[T])

	if !ok {
		return extent.output
	}

	if len(inputs) > 1 {
		if work, workOK := inputs[1].(core.Scalar[T]); workOK {
			sample = core.Scalar[T](T(sample) + T(work))
		}
	}

	extent.output = core.Scalar[T](T(extent.observe(float64(sample))))

	return extent.output
}

/*
ObserveSample ingests one raw sample through the range kernel.
*/
func (extent *Range[T]) ObserveSample(sample T) T {
	derived := T(extent.observe(float64(sample)))
	extent.output = core.Scalar[T](derived)

	return derived
}

/*
ObserveSamples writes one derived value per sample into out.
*/
func (extent *Range[T]) ObserveSamples(samples []T, out []T) {
	for index, sample := range samples {
		out[index] = extent.ObserveSample(sample)
	}
}

/*
Reset clears derived state so the next Observe bootstraps again.
*/
func (extent *Range[T]) Reset() error {
	extent.Min = 0
	extent.Max = 0
	extent.Ready = false
	extent.output = core.Scalar[T](0)

	return nil
}

func (extent *Range[T]) observe(sample float64) float64 {
	if !extent.Ready {
		extent.Min = sample
		extent.Max = sample
		extent.Ready = true

		return 0
	}

	return extent.observeReady(sample)
}

func (extent *Range[T]) observeReady(sample float64) float64 {
	extent.Min = math.Min(extent.Min, sample)
	extent.Max = math.Max(extent.Max, sample)

	return extent.Max - extent.Min
}

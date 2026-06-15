package adaptive

import (
	"math"

	"github.com/theapemachine/nomagique/core"
)

/*
Variance tracks an adaptive mean and variance from the observed sample stream.
*/
type Variance[T ~float64] struct {
	Mean   float64
	Var    float64
	Prev   float64
	Min    float64
	Max    float64
	Rate   float64
	Ready  bool
	output core.Scalar[T]
}

/*
NewVariance returns a variance stage ready to bootstrap from its first observation.
*/
func NewVariance[T ~float64](initial ...core.Number[T]) *Variance[T] {
	variance := &Variance[T]{}

	if len(initial) > 0 {
		variance.output = core.Scalar[T](0).Observe(initial...)
	}

	return variance
}

/*
Observe absorbs the carried sample and returns the variance estimate.
*/
func (variance *Variance[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if len(inputs) == 0 {
		return variance.output
	}

	sample, ok := inputs[0].(core.Scalar[T])

	if !ok {
		return variance.output
	}

	if len(inputs) > 1 {
		if work, workOK := inputs[1].(core.Scalar[T]); workOK {
			sample = core.Scalar[T](T(sample) + T(work))
		}
	}

	variance.output = core.Scalar[T](T(variance.observe(float64(sample))))

	return variance.output
}

/*
ObserveSample ingests one raw sample through the variance kernel.
*/
func (variance *Variance[T]) ObserveSample(sample T) T {
	derived := T(variance.observe(float64(sample)))
	variance.output = core.Scalar[T](derived)

	return derived
}

/*
ObserveSamples writes one derived value per sample into out.
*/
func (variance *Variance[T]) ObserveSamples(samples []T, out []T) {
	for index, sample := range samples {
		out[index] = variance.ObserveSample(sample)
	}
}

/*
Reset clears derived state so the next Observe bootstraps again.
*/
func (variance *Variance[T]) Reset() error {
	variance.Mean = 0
	variance.Var = 0
	variance.Prev = 0
	variance.Min = 0
	variance.Max = 0
	variance.Rate = 0
	variance.Ready = false
	variance.output = core.Scalar[T](0)

	return nil
}

func (variance *Variance[T]) observe(sample float64) float64 {
	if !variance.Ready {
		variance.Mean = sample
		variance.Var = 0
		variance.Prev = sample
		variance.Min = sample
		variance.Max = sample
		variance.Ready = true

		return 0
	}

	return variance.observeReady(sample)
}

func (variance *Variance[T]) observeReady(sample float64) float64 {
	variance.Min = math.Min(variance.Min, sample)
	variance.Max = math.Max(variance.Max, sample)

	span := variance.Max - variance.Min

	if span == 0 {
		variance.Prev = sample

		return variance.Var
	}

	delta := math.Abs(sample - variance.Prev)
	variance.Rate = delta / span
	deviation := sample - variance.Mean
	variance.Mean += variance.Rate * (sample - variance.Mean)
	variance.Var += variance.Rate * (deviation*deviation - variance.Var)
	variance.Prev = sample

	return variance.Var
}

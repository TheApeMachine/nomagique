package adaptive

import (
	"math"

	"github.com/theapemachine/nomagique/core"
)

/*
ZScore tracks adaptive scale for a normalized surprise score.
*/
type ZScore[T ~float64] struct {
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
NewZScore returns a z-score stage ready to bootstrap from its first observation.
*/
func NewZScore[T ~float64](initial ...core.Number[T]) *ZScore[T] {
	surprise := &ZScore[T]{}

	if len(initial) > 0 {
		surprise.output = core.Scalar[T](0).Observe(initial...)
	}

	return surprise
}

/*
Observe absorbs the carried sample and returns the z-score.
*/
func (surprise *ZScore[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if len(inputs) == 0 {
		return surprise.output
	}

	sample, ok := inputs[0].(core.Scalar[T])

	if !ok {
		return surprise.output
	}

	anchor := 0.0
	hasAnchor := false

	if len(inputs) > 1 {
		if anchorSample, anchorOK := inputs[1].(core.Scalar[T]); anchorOK {
			anchor = float64(anchorSample)
			hasAnchor = true
		}
	}

	surprise.output = core.Scalar[T](T(surprise.observe(float64(sample), anchor, hasAnchor)))

	return surprise.output
}

/*
ObserveSample ingests one raw sample through the z-score kernel.
*/
func (surprise *ZScore[T]) ObserveSample(sample T) T {
	derived := T(surprise.observe(float64(sample), 0, false))
	surprise.output = core.Scalar[T](derived)

	return derived
}

/*
ObserveSamples writes one derived value per sample into out.
*/
func (surprise *ZScore[T]) ObserveSamples(samples []T, out []T) {
	for index, sample := range samples {
		out[index] = surprise.ObserveSample(sample)
	}
}

/*
Reset clears derived state so the next Observe bootstraps again.
*/
func (surprise *ZScore[T]) Reset() error {
	surprise.Mean = 0
	surprise.Var = 0
	surprise.Prev = 0
	surprise.Min = 0
	surprise.Max = 0
	surprise.Rate = 0
	surprise.Ready = false
	surprise.output = core.Scalar[T](0)

	return nil
}

func (surprise *ZScore[T]) observe(sample float64, anchorMean float64, hasAnchorMean bool) float64 {
	if !surprise.Ready {
		surprise.Mean = sample
		surprise.Var = 0
		surprise.Prev = sample
		surprise.Min = sample
		surprise.Max = sample
		surprise.Ready = true

		return 0
	}

	return surprise.observeReady(sample, anchorMean, hasAnchorMean)
}

func (surprise *ZScore[T]) observeReady(
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

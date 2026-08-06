package adaptive

import (
	"math"
)

const defaultStandardizerSigmaBound = 8.0

/*
StandardizerConfig configures an online feature standardizer.
*/
type StandardizerConfig struct {
	SigmaBound float64
}

/*
StandardizerOutput reports the standardized z-score along with tracked moments.
*/
type StandardizerOutput struct {
	Value     float64
	Mean      float64
	Variance  float64
	Precision float64
	Ready     bool
	Count     int
}

/*
Standardizer normalizes an incoming feature stream into an adaptive z-score using
Welford's algorithm, scoring each sample against exact prior moments so a transient
cannot dilute its own score.

There is no warmup count. A sample count picked in advance is a claim about how
long every feature takes to become scoreable, which is not something this type
can know and not something any of its callers know either. What it does know is
how precise its own moments are: estimating the centre from n samples inflates
the spread a new sample is drawn from by sqrt(1+1/n), and estimating the spread
itself carries a relative standard error of 1/sqrt(2(n-1)). Scoring against that
predictive scale rather than the raw point estimate makes an early reading small
because the scale it was measured against is uncertain, not because a counter
has not run out, and the two converge as the estimate firms up. A feature is
scoreable the moment it has any spread at all, which is the same condition
ZScore answers to.

Extreme excursions are clamped to a configurable sigma bound.
*/
type Standardizer struct {
	config StandardizerConfig
	mean   float64
	m2     float64
	count  int
}

/*
NewStandardizer returns a typed standardizer configured with sigma bounds.
*/
func NewStandardizer(configs ...StandardizerConfig) *Standardizer {
	config := StandardizerConfig{
		SigmaBound: defaultStandardizerSigmaBound,
	}

	if len(configs) > 0 {
		if configs[0].SigmaBound > 0 {
			config.SigmaBound = configs[0].SigmaBound
		}
	}

	return &Standardizer{
		config: config,
	}
}

/*
Measure scores one sample against prior moments and updates online statistics.
*/
func (standardizer *Standardizer) Measure(sample float64) (StandardizerOutput, error) {
	if err := finiteAdaptive("standardizer", sample); err != nil {
		return StandardizerOutput{}, err
	}

	standardized := 0.0
	priorVariance := standardizer.variance()
	precision := standardizer.precision()
	ready := priorVariance > 0

	if ready {
		scale := math.Sqrt(priorVariance) / precision
		zScore := (sample - standardizer.mean) / scale
		standardized = math.Max(-standardizer.config.SigmaBound, math.Min(standardizer.config.SigmaBound, zScore))
	}

	standardizer.observe(sample)

	return StandardizerOutput{
		Value:     standardized,
		Mean:      standardizer.mean,
		Variance:  standardizer.variance(),
		Precision: precision,
		Ready:     ready,
		Count:     standardizer.count,
	}, nil
}

/*
Precision reports how much of a full z-score the current moments can support,
which is the reciprocal of the inflation a predictive scale carries for having
estimated both the centre and the spread from a finite sample.

It is zero while there is no spread to score against, rises from just under a
half at two observations, and approaches one as the moments settle. A caller
that wants to know whether a feature has warmed asks this instead of counting.
*/
func (standardizer *Standardizer) Precision() float64 {
	return standardizer.precision()
}

func (standardizer *Standardizer) precision() float64 {
	if standardizer.count < 2 || standardizer.variance() <= 0 {
		return 0
	}

	count := float64(standardizer.count)
	centre := math.Sqrt(1 + 1/count)
	spread := 1 + 1/math.Sqrt(2*(count-1))

	return 1 / (centre * spread)
}

/*
Standardize processes one sample directly and returns the standardized value.
*/
func (standardizer *Standardizer) Standardize(sample float64) float64 {
	output, err := standardizer.Measure(sample)

	if err != nil {
		return 0
	}

	return output.Value
}

/*
Count returns the number of observed samples.
*/
func (standardizer *Standardizer) Count() int {
	return standardizer.count
}

/*
Reset clears the tracked moments and count.
*/
func (standardizer *Standardizer) Reset() {
	standardizer.mean = 0
	standardizer.m2 = 0
	standardizer.count = 0
}

func (standardizer *Standardizer) observe(sample float64) {
	standardizer.count++
	delta := sample - standardizer.mean
	standardizer.mean += delta / float64(standardizer.count)
	delta2 := sample - standardizer.mean
	standardizer.m2 += delta * delta2
}

func (standardizer *Standardizer) variance() float64 {
	if standardizer.count < 2 {
		return 0
	}

	return standardizer.m2 / float64(standardizer.count-1)
}

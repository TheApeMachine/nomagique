package adaptive

import (
	"math"
)

const (
	defaultStandardizerWarmup     = 32
	defaultStandardizerSigmaBound = 8.0
)

/*
StandardizerConfig configures an online feature standardizer.
*/
type StandardizerConfig struct {
	Warmup     int
	SigmaBound float64
}

/*
StandardizerOutput reports the standardized z-score along with tracked moments.
*/
type StandardizerOutput struct {
	Value    float64
	Mean     float64
	Variance float64
	Ready    bool
	Count    int
}

/*
Standardizer normalizes an incoming feature stream into an adaptive z-score using
Welford's algorithm, scoring each sample against exact prior moments so a transient
cannot dilute its own score. Output remains centered at zero until a minimum warmup
sample count has been observed, and extreme excursions are clamped to a configurable
sigma bound.
*/
type Standardizer struct {
	config StandardizerConfig
	mean   float64
	m2     float64
	count  int
}

/*
NewStandardizer returns a typed standardizer configured with warmup and sigma bounds.
*/
func NewStandardizer(configs ...StandardizerConfig) *Standardizer {
	config := StandardizerConfig{
		Warmup:     defaultStandardizerWarmup,
		SigmaBound: defaultStandardizerSigmaBound,
	}

	if len(configs) > 0 {
		if configs[0].Warmup > 0 {
			config.Warmup = configs[0].Warmup
		}

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
	ready := standardizer.count >= standardizer.config.Warmup

	if ready && priorVariance > 0 {
		deviation := math.Sqrt(priorVariance)
		zScore := (sample - standardizer.mean) / deviation
		standardized = math.Max(-standardizer.config.SigmaBound, math.Min(standardizer.config.SigmaBound, zScore))
	}

	standardizer.observe(sample)

	return StandardizerOutput{
		Value:    standardized,
		Mean:     standardizer.mean,
		Variance: standardizer.variance(),
		Ready:    ready,
		Count:    standardizer.count,
	}, nil
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

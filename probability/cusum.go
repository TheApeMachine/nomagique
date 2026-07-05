package probability

import (
	"math"

	"github.com/theapemachine/errnie"
)

/*
CUSUMConfig configures sequential change evidence.
*/
type CUSUMConfig struct {
	Reference float64
}

/*
CUSUMOutput reports signed and absolute sequential change evidence.
*/
type CUSUMOutput struct {
	Value    float64
	Positive float64
	Negative float64
	Ready    bool
	Count    int
}

/*
CUSUM accumulates sequential change evidence from a sample stream.
*/
type CUSUM struct {
	config   CUSUMConfig
	target   float64
	positive float64
	negative float64
	prev     float64
	minimum  float64
	maximum  float64
	count    int
}

/*
NewCUSUM returns a typed change detector.
*/
func NewCUSUM(configs ...CUSUMConfig) *CUSUM {
	config := CUSUMConfig{}

	if len(configs) > 0 {
		config = configs[0]
	}

	return &CUSUM{
		config: config,
	}
}

/*
Measure adds one sample and returns sequential change evidence.
*/
func (cusum *CUSUM) Measure(sample float64) (CUSUMOutput, error) {
	if err := finiteProbability("cusum", sample); err != nil {
		return CUSUMOutput{}, err
	}

	if cusum.config.Reference < 0 ||
		math.IsNaN(cusum.config.Reference) ||
		math.IsInf(cusum.config.Reference, 0) {
		return CUSUMOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"cusum: reference must be finite and non-negative",
			nil,
		))
	}

	if cusum.count == 0 {
		cusum.minimum = sample
		cusum.maximum = sample
		cusum.prev = sample
		cusum.target = sample
		cusum.positive = 0
		cusum.negative = 0
		cusum.count = 1

		return CUSUMOutput{
			Ready: false,
			Count: cusum.count,
		}, nil
	}

	cusum.minimum = math.Min(cusum.minimum, sample)
	cusum.maximum = math.Max(cusum.maximum, sample)
	cusum.count++
	cusum.positive = math.Max(0, cusum.positive+sample-cusum.target-cusum.config.Reference)
	cusum.negative = math.Max(0, cusum.negative+cusum.target-sample-cusum.config.Reference)
	cusum.prev = sample
	value := math.Max(cusum.positive, cusum.negative)

	return CUSUMOutput{
		Value:    value,
		Positive: cusum.positive,
		Negative: cusum.negative,
		Ready:    value > 0,
		Count:    cusum.count,
	}, nil
}

/*
Reset clears accumulated change evidence.
*/
func (cusum *CUSUM) Reset() {
	cusum.target = 0
	cusum.positive = 0
	cusum.negative = 0
	cusum.prev = 0
	cusum.minimum = 0
	cusum.maximum = 0
	cusum.count = 0
}

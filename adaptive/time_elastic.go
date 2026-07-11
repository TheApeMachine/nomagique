package adaptive

import (
	"math"
	"time"

	"github.com/theapemachine/errnie"
)

/*
TimeElasticConfig configures time-decayed baseline tracking.
*/
type TimeElasticConfig struct {
	Halflife time.Duration
	Epsilon  float64
}

/*
TimedValue carries a sample and its event time.
*/
type TimedValue struct {
	Value float64
	At    time.Time
}

/*
TimeElastic tracks a time-decayed baseline and returns sample/baseline ratios.
*/
type TimeElastic struct {
	config   TimeElasticConfig
	baseline float64
	lastAt   time.Time
	ready    bool
	count    int
}

/*
TimeElasticOutput reports the sample-to-baseline ratio.
*/
type TimeElasticOutput struct {
	Value float64
	Ready bool
	Count int
}

/*
NewTimeElastic returns a typed time-elastic baseline tracker.
*/
func NewTimeElastic(config TimeElasticConfig) *TimeElastic {
	return &TimeElastic{
		config: config,
	}
}

/*
Measure adds one timed sample and returns sample/baseline ratio when ready.
*/
func (timeElastic *TimeElastic) Measure(sample TimedValue) (TimeElasticOutput, error) {
	if timeElastic.config.Halflife <= 0 || timeElastic.config.Epsilon <= 0 {
		return TimeElasticOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"time-elastic: halflife and epsilon must be positive",
			nil,
		))
	}

	if err := finiteAdaptive("time-elastic", sample.Value); err != nil {
		return TimeElasticOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"time-elastic: value must be finite",
			err,
		))
	}

	if sample.Value < 0 {
		return TimeElasticOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"time-elastic: value must be non-negative",
			nil,
		))
	}

	if sample.At.IsZero() {
		return TimeElasticOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"time-elastic: event timestamp required",
			nil,
		))
	}

	if !timeElastic.ready {
		timeElastic.baseline = sample.Value
		timeElastic.lastAt = sample.At
		timeElastic.ready = true
		timeElastic.count = 1

		// The first reading seeds the baseline with itself, so the
		// sample-to-baseline ratio is the reflexive identity: no
		// deviation observed yet, which is a defined value, not a
		// missing one.
		return TimeElasticOutput{
			Value: 1,
			Ready: true,
			Count: timeElastic.count,
		}, nil
	}

	delta := sample.At.Sub(timeElastic.lastAt)

	if delta < 0 {
		return TimeElasticOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"time-elastic: event timestamp must not regress",
			nil,
		))
	}

	value := sample.Value / (timeElastic.baseline + timeElastic.config.Epsilon)
	tau := float64(timeElastic.config.Halflife) / math.Ln2
	alpha := 0.0

	if delta > 0 {
		alpha = 1.0 - math.Exp(-float64(delta)/tau)
	}

	timeElastic.baseline = (1.0-alpha)*timeElastic.baseline + alpha*sample.Value
	timeElastic.lastAt = sample.At
	timeElastic.count++

	if err := finiteAdaptive("time-elastic", value); err != nil {
		return TimeElasticOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"time-elastic: value must be finite",
			err,
		))
	}

	return TimeElasticOutput{
		Value: value,
		Ready: delta > 0,
		Count: timeElastic.count,
	}, nil
}

/*
Baseline returns the current time-decayed scale estimate.
*/
func (timeElastic *TimeElastic) Baseline() float64 {
	if !timeElastic.ready {
		return 0
	}

	return timeElastic.baseline
}

/*
Ready reports whether at least one timed sample established a baseline.
*/
func (timeElastic *TimeElastic) Ready() bool {
	return timeElastic.ready
}

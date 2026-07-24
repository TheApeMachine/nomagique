package adaptive

import (
	"math"
	"time"

	"github.com/theapemachine/errnie"
)

/*
LogReturnConfig configures lagged log-return history retention.
*/
type LogReturnConfig struct {
	ReturnLag int
	MaxSeries int
}

/*
LogReturnSample carries a positive sample with event time and series key.
*/
type LogReturnSample struct {
	Series string
	Value  float64
	At     time.Time
}

/*
LogReturn computes a lagged log return from a fixed-size ring per series.
*/
type LogReturn struct {
	config  LogReturnConfig
	samples map[string]*logReturnRing
}

type logReturnRing struct {
	values []LogReturnSample
	head   int
	count  int
}

/*
LogReturnOutput reports a lagged log return.
*/
type LogReturnOutput struct {
	Value float64
	Ready bool
	Count int
}

/*
NewLogReturn returns a typed log-return calculator.
The ring holds exactly ReturnLag+1 observations so memory stays bounded by the
lag that the statistic actually needs.
*/
func NewLogReturn(configs ...LogReturnConfig) (*LogReturn, error) {
	config := LogReturnConfig{}

	if len(configs) > 0 {
		config = configs[0]
	}

	if config.ReturnLag <= 0 {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"log-return: positive return lag required",
			nil,
		))
	}

	if config.MaxSeries <= 0 {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"log-return: positive max series required",
			nil,
		))
	}

	return &LogReturn{
		config:  config,
		samples: map[string]*logReturnRing{},
	}, nil
}

/*
Measure adds one sample and returns the lagged log return when the lag is full.
*/
func (logReturn *LogReturn) Measure(sample LogReturnSample) (LogReturnOutput, error) {
	if err := finiteAdaptive("log-return", sample.Value); err != nil {
		return LogReturnOutput{}, err
	}

	if sample.Value <= 0 {
		return LogReturnOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"log-return: value must be positive",
			nil,
		))
	}

	if sample.At.IsZero() {
		return LogReturnOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"log-return: event timestamp required",
			nil,
		))
	}

	series := sample.Series

	if series == "" {
		series = "default"
	}

	ring, err := logReturn.ring(series)

	if err != nil {
		return LogReturnOutput{}, err
	}

	if ring.count > 0 {
		latest := ring.latest()

		if sample.At.Before(latest.At) {
			return LogReturnOutput{}, errnie.Error(errnie.Err(
				errnie.Validation,
				"log-return: event timestamp must not regress",
				nil,
			))
		}
	}

	sample.Series = series
	ring.push(sample)
	capacity := logReturn.config.ReturnLag + 1

	if ring.count <= logReturn.config.ReturnLag {
		return LogReturnOutput{
			Value: 0,
			Ready: false,
			Count: ring.count,
		}, nil
	}

	anchor := ring.atLag(logReturn.config.ReturnLag)

	if anchor.Value <= 0 {
		return LogReturnOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"log-return: anchor value must be positive",
			nil,
		))
	}

	value := math.Log(sample.Value / anchor.Value)

	if err := finiteAdaptive("log-return", value); err != nil {
		return LogReturnOutput{}, err
	}

	return LogReturnOutput{
		Value: value,
		Ready: true,
		Count: min(ring.count, capacity),
	}, nil
}

func (logReturn *LogReturn) ring(series string) (*logReturnRing, error) {
	existing, ok := logReturn.samples[series]

	if ok {
		return existing, nil
	}

	if len(logReturn.samples) >= logReturn.config.MaxSeries {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"log-return: series cardinality exceeded",
			nil,
		))
	}

	capacity := logReturn.config.ReturnLag + 1
	ring := &logReturnRing{
		values: make([]LogReturnSample, capacity),
	}
	logReturn.samples[series] = ring

	return ring, nil
}

func (ring *logReturnRing) push(sample LogReturnSample) {
	if ring.count < len(ring.values) {
		ring.values[ring.count] = sample
		ring.head = ring.count
		ring.count++

		return
	}

	ring.head = (ring.head + 1) % len(ring.values)
	ring.values[ring.head] = sample
	ring.count++
}

func (ring *logReturnRing) latest() LogReturnSample {
	return ring.values[ring.head]
}

func (ring *logReturnRing) atLag(lag int) LogReturnSample {
	capacity := len(ring.values)
	index := ring.head - lag

	if index < 0 {
		index += capacity
	}

	return ring.values[index]
}

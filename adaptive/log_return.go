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
	ReturnLag  int
	LongWindow int
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
LogReturn computes a lagged log return from a retained sample series.
*/
type LogReturn struct {
	config  LogReturnConfig
	samples map[string][]LogReturnSample
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
*/
func NewLogReturn(configs ...LogReturnConfig) *LogReturn {
	config := LogReturnConfig{
		ReturnLag: 1,
	}

	if len(configs) > 0 {
		config = configs[0]
	}

	if config.ReturnLag <= 0 {
		config.ReturnLag = 1
	}

	return &LogReturn{
		config:  config,
		samples: map[string][]LogReturnSample{},
	}
}

/*
Measure adds one sample and returns the lagged log return.
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

	history := logReturn.samples[series]

	if len(history) > 0 && sample.At.Before(history[len(history)-1].At) {
		return LogReturnOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"log-return: event timestamp must not regress",
			nil,
		))
	}

	sample.Series = series
	history = append(history, sample)
	history = logReturn.trim(history)
	logReturn.samples[series] = history
	anchorIndex := len(history) - logReturn.config.ReturnLag - 1

	if anchorIndex < 0 {
		// No anchor lagged far enough back yet. The reflexive boundary
		// (comparing the sample to itself) is a defined "no return
		// observed yet" value, not a missing one.
		return LogReturnOutput{
			Value: 0,
			Ready: true,
			Count: len(history),
		}, nil
	}

	anchor := history[anchorIndex].Value

	if anchor <= 0 {
		return LogReturnOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"log-return: anchor value must be positive",
			nil,
		))
	}

	value := math.Log(sample.Value / anchor)

	if err := finiteAdaptive("log-return", value); err != nil {
		return LogReturnOutput{}, err
	}

	return LogReturnOutput{
		Value: value,
		Ready: true,
		Count: len(history),
	}, nil
}

func (logReturn *LogReturn) trim(history []LogReturnSample) []LogReturnSample {
	keep := logReturn.config.LongWindow + logReturn.config.ReturnLag

	if keep <= logReturn.config.ReturnLag {
		return history
	}

	if len(history) <= keep {
		return history
	}

	return history[len(history)-keep:]
}

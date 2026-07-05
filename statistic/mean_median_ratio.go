package statistic

import (
	"math"
	"time"

	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
MeanMedianRatioConfig configures the adaptive ratio calculator.
*/
type MeanMedianRatioConfig struct {
	ShortWindow int
	LongWindow  int
	Transform   string
}

/*
MeanMedianRatioOutput reports ratio plus optional decline pressure.
*/
type MeanMedianRatioOutput struct {
	Value   float64
	Decline float64
	Ready   bool
	Count   int
}

/*
MeanMedianRatio compares a short-window mean to a long-window median on streamed samples.
*/
type MeanMedianRatio struct {
	config          MeanMedianRatioConfig
	histories       map[string][]TimedSample
	previousSamples map[string]float64
	previousDeltas  map[string]float64
}

/*
NewMeanMedianRatio returns a typed mean-over-median ratio accumulator.
*/
func NewMeanMedianRatio(configs ...MeanMedianRatioConfig) *MeanMedianRatio {
	config := MeanMedianRatioConfig{}

	if len(configs) > 0 {
		config = configs[0]
	}

	return &MeanMedianRatio{
		config:          config,
		histories:       map[string][]TimedSample{},
		previousSamples: map[string]float64{},
		previousDeltas:  map[string]float64{},
	}
}

/*
Measure adds one timed sample and returns short-mean divided by prior long median.
*/
func (meanMedianRatio *MeanMedianRatio) Measure(sample TimedSample) (MeanMedianRatioOutput, error) {
	if err := finiteStatistic("mean-median-ratio", sample.Value); err != nil {
		return MeanMedianRatioOutput{}, err
	}

	if sample.At.IsZero() {
		return MeanMedianRatioOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"mean-median-ratio: event timestamp required",
			nil,
		))
	}

	series := sample.Series

	if series == "" {
		series = "default"
	}

	value := meanMedianRatio.transform(series, sample.Value)
	history := meanMedianRatio.histories[series]

	if len(history) > 0 && sample.At.Before(history[len(history)-1].At) {
		return MeanMedianRatioOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"mean-median-ratio: event timestamp must not regress",
			nil,
		))
	}

	decline := meanMedianRatio.decline(series, value)
	history = append(history, TimedSample{
		Series: series,
		Value:  value,
		At:     sample.At,
	})

	history = meanMedianRatio.trim(history)
	meanMedianRatio.histories[series] = history

	output, err := meanMedianRatio.ratio(history)
	if err != nil {
		return MeanMedianRatioOutput{}, err
	}

	output.Decline = decline

	return output, nil
}

func (meanMedianRatio *MeanMedianRatio) transform(series string, sample float64) float64 {
	if meanMedianRatio.config.Transform == "" {
		return sample
	}

	previousSample := meanMedianRatio.previousSamples[series]
	meanMedianRatio.previousSamples[series] = sample

	if previousSample <= 0 {
		return sample
	}

	delta := sample - previousSample

	if meanMedianRatio.config.Transform == "delta" {
		return delta
	}

	if meanMedianRatio.config.Transform == "deltaPositive" && delta > 0 {
		return delta
	}

	if meanMedianRatio.config.Transform == "deltaPositive" {
		return 0
	}

	return sample
}

func (meanMedianRatio *MeanMedianRatio) decline(series string, sample float64) float64 {
	previousDelta := meanMedianRatio.previousDeltas[series]
	meanMedianRatio.previousDeltas[series] = sample

	if previousDelta <= 0 || sample >= previousDelta {
		return 0
	}

	return (previousDelta - sample) / previousDelta
}

func (meanMedianRatio *MeanMedianRatio) trim(history []TimedSample) []TimedSample {
	longWindow := meanMedianRatio.config.LongWindow

	if longWindow <= 0 {
		values := timedValues(history)
		resolved, err := ResolveWindowSet(values, WindowsConfig{
			ShortHint: meanMedianRatio.config.ShortWindow,
			LongHint:  meanMedianRatio.config.LongWindow,
		})

		if err == nil {
			longWindow = resolved.LongWindow
		}
	}

	if longWindow <= 0 || len(history) <= longWindow {
		return history
	}

	return history[len(history)-longWindow:]
}

func (meanMedianRatio *MeanMedianRatio) ratio(history []TimedSample) (MeanMedianRatioOutput, error) {
	values := timedValues(history)
	window, err := ResolveWindowSet(values, WindowsConfig{
		ShortHint: meanMedianRatio.config.ShortWindow,
		LongHint:  meanMedianRatio.config.LongWindow,
	})

	if err != nil {
		return MeanMedianRatioOutput{}, err
	}

	shortWindow := window.ShortWindow

	if shortWindow > len(values) {
		shortWindow = len(values)
	}

	if shortWindow <= 0 {
		return MeanMedianRatioOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"mean-median-ratio: short window is empty",
			nil,
		))
	}

	shortSlice := values[len(values)-shortWindow:]
	shortMean := stat.Mean(shortSlice, nil)
	longSlice := values

	if len(values) > shortWindow {
		longSlice = values[:len(values)-shortWindow]
	}

	longMedian, ok := MedianOf(longSlice)

	if (!ok || longMedian <= 0) && meanMedianRatio.config.Transform == "deltaPositive" {
		longMedian, ok = MedianOf(positiveValues(longSlice))
	}

	if !ok || longMedian <= 0 {
		longMedian = shortMean
	}

	ratio := 0.0

	if longMedian > 0 {
		ratio = shortMean / longMedian
	}

	if math.IsNaN(ratio) || math.IsInf(ratio, 0) {
		return MeanMedianRatioOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"mean-median-ratio: output value is non-finite",
			nil,
		))
	}

	return MeanMedianRatioOutput{
		Value: ratio,
		Ready: true,
		Count: len(values),
	}, nil
}

func timedValues(history []TimedSample) []float64 {
	values := make([]float64, len(history))

	for index, point := range history {
		values[index] = point.Value
	}

	return values
}

func positiveValues(values []float64) []float64 {
	positive := make([]float64, 0, len(values))

	for _, value := range values {
		if value > 0 {
			positive = append(positive, value)
		}
	}

	return positive
}

func timestamp(seconds int64) time.Time {
	return time.Unix(seconds, 0)
}

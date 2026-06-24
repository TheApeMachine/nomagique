package statistic

import (
	"math"
	"time"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
MeanMedianRatio compares a short-window mean to a long-window median on streamed samples.
*/
type MeanMedianRatio struct {
	artifact  *datura.Artifact
	histories map[string][]struct {
		value float64
		at    time.Time
	}
	previousSamples map[string]float64
	previousDeltas  map[string]float64
	declines        map[string]float64
}

/*
NewMeanMedianRatio returns a mean-over-median ratio stage wired from config attributes on the artifact.
*/
func NewMeanMedianRatio(artifact *datura.Artifact) *MeanMedianRatio {
	return &MeanMedianRatio{
		artifact: artifact,
		histories: map[string][]struct {
			value float64
			at    time.Time
		}{},
		previousSamples: map[string]float64{},
		previousDeltas:  map[string]float64{},
		declines:        map[string]float64{},
	}
}

func (meanMedianRatio *MeanMedianRatio) Read(payload []byte) (int, error) {
	state := datura.Acquire("mean-median-ratio-state", datura.APPJSON)

	if _, err := state.Write(meanMedianRatio.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	state.Inspect("statistic", "mean-median-ratio", "Read()", "p")

	root := datura.Peek[string](state, "root")
	inputs := datura.Peek[[]string](state, "inputs")
	features := datura.Peek[[]float64](state, "features")
	featureInputs := datura.Peek[[]string](state, "featureInputs")
	featureRoot := datura.Peek[string](state, "root")

	if len(featureInputs) == 0 && featureRoot == "features" {
		featureInputs = inputs
	}

	order := datura.Peek[[]string](meanMedianRatio.artifact, "order")
	stageIndex := int(datura.Peek[float64](meanMedianRatio.artifact, "stageIndex"))
	stageKey := ""

	if stageIndex < 0 {
		stageIndex = 0
	}

	if len(order) > stageIndex {
		stageKey = order[stageIndex]
	}

	if stageKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"mean-median-ratio: stage order required",
			nil,
		))
	}

	sourceKey := datura.Peek[string](meanMedianRatio.artifact, stageKey, "input")

	if sourceKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"mean-median-ratio: input key required",
			nil,
		))
	}

	var sample float64
	sampleFound := false

	for index, key := range inputs {
		if key != sourceKey || index >= len(features) {
			continue
		}

		sample = features[index]
		sampleFound = true
	}

	if !sampleFound {
		for index, key := range featureInputs {
			if key != sourceKey || index >= len(features) {
				continue
			}

			sample = features[index]
			sampleFound = true
		}
	}

	if !sampleFound && root != "" {
		sample = datura.Peek[float64](state, root, sourceKey)
		sampleFound = true
	}

	if !sampleFound {
		sample = datura.Peek[float64](state, sourceKey)
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"mean-median-ratio: sample is non-finite",
			nil,
		))
	}

	seriesKey := stageKey
	scope, _ := state.Scope()

	if scope != "" {
		seriesKey = stageKey + "/" + scope
	}

	timestamp := state.Timestamp()

	if timestamp <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"mean-median-ratio: event timestamp required",
			nil,
		))
	}

	observed := time.Unix(0, timestamp)
	transform := datura.Peek[string](meanMedianRatio.artifact, stageKey, "transform")

	if transform != "" {
		previousSample := meanMedianRatio.previousSamples[seriesKey]
		meanMedianRatio.previousSamples[seriesKey] = sample

		if transform == "delta" && previousSample > 0 {
			sample = sample - previousSample
		}

		if transform == "deltaPositive" && previousSample > 0 {
			delta := sample - previousSample

			if delta < 0 {
				sample = 0
			}

			if delta >= 0 {
				sample = delta
			}
		}
	}

	declineOutput := datura.Peek[string](meanMedianRatio.artifact, stageKey, "decline", "output")

	if declineOutput != "" {
		previousDelta := meanMedianRatio.previousDeltas[seriesKey]
		decline := 0.0

		if previousDelta > 0 && sample < previousDelta {
			decline = (previousDelta - sample) / previousDelta
		}

		meanMedianRatio.previousDeltas[seriesKey] = sample
		meanMedianRatio.declines[seriesKey+"/"+declineOutput] = decline
	}

	shortHint := int(datura.Peek[float64](meanMedianRatio.artifact, stageKey, "shortWindow"))
	longHint := int(datura.Peek[float64](meanMedianRatio.artifact, stageKey, "longWindow"))
	outputKey := datura.Peek[string](meanMedianRatio.artifact, stageKey, "outputKey")

	if outputKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"mean-median-ratio: outputKey required",
			nil,
		))
	}

	history := meanMedianRatio.histories[seriesKey]

	if len(history) > 0 && observed.Before(history[len(history)-1].at) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"mean-median-ratio: event timestamp must not regress",
			nil,
		))
	}

	history = append(history, struct {
		value float64
		at    time.Time
	}{value: sample, at: observed})

	shortWindow := shortHint
	longWindow := longHint

	if longWindow <= 0 {
		longWindow = len(history)
	}

	if shortWindow <= 0 {
		shortWindow = longWindow

		if shortWindow < 1 {
			shortWindow = 1
		}
	}

	if shortWindow > longWindow {
		shortWindow = longWindow
	}

	if longWindow > 0 && len(history) > longWindow {
		history = history[len(history)-longWindow:]
	}

	meanMedianRatio.histories[seriesKey] = history

	shortCount := min(shortWindow, len(history))
	values := make([]float64, len(history))

	for index, point := range history {
		values[index] = point.value
	}

	shortSlice := values[len(values)-shortCount:]
	shortMean := stat.Mean(shortSlice, nil)
	longSlice := values

	if len(values) > shortCount {
		longSlice = values[:len(values)-shortCount]
	}

	longMedian, ok := MedianOf(longSlice)

	if (!ok || longMedian <= 0) && transform == "deltaPositive" {
		positiveLong := make([]float64, 0, len(longSlice))

		for _, value := range longSlice {
			if value > 0 {
				positiveLong = append(positiveLong, value)
			}
		}

		longMedian, ok = MedianOf(positiveLong)
	}

	if !ok || longMedian <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"mean-median-ratio: long median is invalid",
			nil,
		))
	}

	ratio := shortMean / longMedian

	if len(features) > 0 {
		state.Merge("features", features)
	}

	if len(featureInputs) > 0 {
		state.Poke("features", "root")
		state.Poke(featureInputs, "inputs")
		state.Poke(featureInputs, "featureInputs")
	}

	if len(featureInputs) == 0 {
		if featureRoot != "" {
			state.Poke(featureRoot, "root")
		}

		if len(inputs) > 0 {
			state.Poke(inputs, "inputs")
		}
	}

	state.MergeOutput(outputKey, ratio)

	if declineOutput != "" {
		state.MergeOutput(declineOutput, meanMedianRatio.declines[seriesKey+"/"+declineOutput])
	}

	return state.Read(payload)
}

func (meanMedianRatio *MeanMedianRatio) Write(payload []byte) (int, error) {
	meanMedianRatio.artifact.WithPayload(payload)
	return len(payload), nil
}

func (meanMedianRatio *MeanMedianRatio) Close() error {
	return nil
}

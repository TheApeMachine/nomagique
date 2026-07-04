package statistic

import (
	"io"
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
	artifact     *datura.Artifact
	pendingFrame bool
	output       []byte
	histories    map[string][]struct {
		value float64
		at    time.Time
	}
	previousSamples map[string]float64
	previousDeltas  map[string]float64
	declines        map[string]float64
}

/*
NewMeanMedianRatio returns a mean-over-median ratio stage wired from config
attributes on the artifact. The stage's config block is named by the "block"
attribute, the same convention price_ring uses.
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
	if len(meanMedianRatio.output) > 0 {
		return meanMedianRatio.readOutput(payload)
	}

	if !meanMedianRatio.pendingFrame {
		return 0, io.EOF
	}

	state := datura.Acquire("mean-median-ratio-state", datura.APPJSON)
	frame := meanMedianRatio.artifact.DecryptPayload()

	if len(frame) == 0 {
		state.Release()
		meanMedianRatio.pendingFrame = false

		return 0, io.EOF
	}

	if _, err := state.Unpack(frame); err != nil {
		state.Release()
		meanMedianRatio.pendingFrame = false

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"mean-median-ratio: state write failed",
			err,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")
	features := datura.Peek[[]float64](state, "features")
	featureRoot := datura.Peek[string](state, "root")

	stageKey := stageConfigKey(meanMedianRatio.artifact, datura.Peek[string](meanMedianRatio.artifact, "block"))
	sourceKey := configString(meanMedianRatio.artifact, stageKey, "input")

	if sourceKey == "" {
		state.Release()
		meanMedianRatio.pendingFrame = false

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
		state.Release()
		meanMedianRatio.pendingFrame = false

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"mean-median-ratio: source key not found in inputs",
			nil,
		))
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		state.Release()
		meanMedianRatio.pendingFrame = false

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"mean-median-ratio: sample is non-finite",
			nil,
		))
	}

	seriesKey := stageKey

	if seriesKey == "" {
		seriesKey = sourceKey
	}

	scope, _ := state.Scope()

	if scope != "" {
		seriesKey = stageKey + "/" + scope
	}

	timestamp := state.Timestamp()

	if timestamp <= 0 {
		state.Release()
		meanMedianRatio.pendingFrame = false

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"mean-median-ratio: event timestamp required",
			nil,
		))
	}

	observed := time.Unix(0, timestamp)
	transform := configString(meanMedianRatio.artifact, stageKey, "transform")

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

	declineOutput := configNestedString(meanMedianRatio.artifact, stageKey, "decline", "output")

	if declineOutput != "" {
		previousDelta := meanMedianRatio.previousDeltas[seriesKey]
		decline := 0.0

		if previousDelta > 0 && sample < previousDelta {
			decline = (previousDelta - sample) / previousDelta
		}

		meanMedianRatio.previousDeltas[seriesKey] = sample
		meanMedianRatio.declines[seriesKey+"/"+declineOutput] = decline
	}

	shortHint := int(configFloat(meanMedianRatio.artifact, stageKey, "shortWindow"))

	if shortHint <= 0 {
		shortHint = int(datura.Peek[float64](state, "output", "shortWindow"))
	}

	longHint := int(configFloat(meanMedianRatio.artifact, stageKey, "longWindow"))

	if longHint <= 0 {
		longHint = int(datura.Peek[float64](state, "output", "longWindow"))
	}
	outputKey := configString(meanMedianRatio.artifact, stageKey, "outputKey")

	if outputKey == "" {
		state.Release()
		meanMedianRatio.pendingFrame = false

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"mean-median-ratio: outputKey required",
			nil,
		))
	}

	history := meanMedianRatio.histories[seriesKey]

	if len(history) > 0 && observed.Before(history[len(history)-1].at) {
		state.Release()
		meanMedianRatio.pendingFrame = false

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

	ratio := 0.0
	if !ok || longMedian <= 0 {
		longMedian = shortMean
	}
	if longMedian > 0 {
		ratio = shortMean / longMedian
	}

	if len(features) > 0 {
		state.Merge("features", features)
	}

	if featureRoot != "" {
		state.Poke(featureRoot, "root")
	}

	if len(inputs) > 0 {
		state.Poke(inputs, "inputs")
		state.Poke(inputs, "featureInputs")
	}

	state.MergeOutput(outputKey, ratio)

	if declineOutput != "" {
		state.MergeOutput(declineOutput, meanMedianRatio.declines[seriesKey+"/"+declineOutput])
	}

	outputInputs := datura.Peek[[]string](state, "inputs")

	if outputInputs == nil {
		outputInputs = []string{}
	}

	outputInputs = append(outputInputs, outputKey)

	if declineOutput != "" {
		outputInputs = append(outputInputs, declineOutput)
	}

	state.Poke("output", "root")
	state.Poke(outputInputs, "inputs")

	meanMedianRatio.output = state.Pack()
	state.Release()

	return meanMedianRatio.readOutput(payload)
}

func (meanMedianRatio *MeanMedianRatio) Write(payload []byte) (int, error) {
	if len(payload) == 0 {
		meanMedianRatio.pendingFrame = false
		meanMedianRatio.output = nil

		return 0, nil
	}

	meanMedianRatio.artifact.WithPayload(payload)
	meanMedianRatio.pendingFrame = true
	meanMedianRatio.output = nil

	return len(payload), nil
}

func (meanMedianRatio *MeanMedianRatio) readOutput(payload []byte) (int, error) {
	n := copy(payload, meanMedianRatio.output)

	if n < len(meanMedianRatio.output) {
		return n, io.ErrShortBuffer
	}

	meanMedianRatio.output = nil
	meanMedianRatio.pendingFrame = false

	return n, io.EOF
}

func (meanMedianRatio *MeanMedianRatio) Close() error {
	return nil
}

func stageConfigKey(artifact *datura.Artifact, explicit string) string {
	if explicit != "" {
		return explicit
	}

	order := datura.Peek[[]string](artifact, "order")
	stageIndex := int(datura.Peek[float64](artifact, "stageIndex"))

	if stageIndex >= 0 && stageIndex < len(order) {
		return order[stageIndex]
	}

	return ""
}

func configString(artifact *datura.Artifact, block, key string) string {
	if block != "" {
		return datura.Peek[string](artifact, block, key)
	}

	return datura.Peek[string](artifact, key)
}

func configNestedString(artifact *datura.Artifact, block, key, nested string) string {
	if block != "" {
		return datura.Peek[string](artifact, block, key, nested)
	}

	return datura.Peek[string](artifact, key, nested)
}

func configFloat(artifact *datura.Artifact, block, key string) float64 {
	if block != "" {
		return datura.Peek[float64](artifact, block, key)
	}

	return datura.Peek[float64](artifact, key)
}

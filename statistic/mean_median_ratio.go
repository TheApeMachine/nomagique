package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
MeanMedianRatio compares a short-window mean to a long-window median on streamed samples.
The constructor artifact holds config; runtime history lives on the stage instance.
*/
type MeanMedianRatio struct {
	artifact        *datura.Artifact
	histories       map[string][]Observation
	previousSamples map[string]float64
	previousDeltas  map[string]float64
	declines        map[string]float64
}

/*
NewMeanMedianRatio returns a mean-over-median ratio stage wired from config attributes on the artifact.
*/
func NewMeanMedianRatio(artifact *datura.Artifact) *MeanMedianRatio {
	return &MeanMedianRatio{
		artifact:        artifact,
		histories:       map[string][]Observation{},
		previousSamples: map[string]float64{},
		previousDeltas:  map[string]float64{},
		declines:        map[string]float64{},
	}
}

func (meanMedianRatio *MeanMedianRatio) Write(payload []byte) (int, error) {
	meanMedianRatio.artifact.WithPayload(payload)
	return len(payload), nil
}

func (meanMedianRatio *MeanMedianRatio) Read(payload []byte) (int, error) {
	state := datura.Acquire("mean-median-ratio-state", datura.APPJSON)

	if _, err := state.Write(meanMedianRatio.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	state.Inspect("statistic", "mean-median-ratio", "Read()", "p")

	features := SnapshotFeatures(state)
	stageKey := meanMedianRatio.stageKey()

	if stageKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"mean-median-ratio: stage order required",
			nil,
		))
	}

	sourceKey := datura.Peek[string](meanMedianRatio.artifact, stageKey, "input")
	sample, err := meanMedianRatio.sample(state, sourceKey)

	if err != nil {
		return 0, err
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"mean-median-ratio: sample is non-finite",
			nil,
		))
	}

	seriesKey := SeriesKey(meanMedianRatio.artifact, state, stageKey)
	observed, err := EventTime(meanMedianRatio.artifact, state)

	if err != nil {
		return 0, err
	}

	sample = meanMedianRatio.applyTransform(seriesKey, stageKey, sample)
	meanMedianRatio.trackDecline(seriesKey, stageKey, sample)

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

	history, err := AppendObservation(meanMedianRatio.histories[seriesKey], sample, observed)

	if err != nil {
		return 0, err
	}

	shortWindow, longWindow, err := RollingObservationWindows(history, shortHint, longHint)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"mean-median-ratio: unable to resolve windows",
			err,
		))
	}

	if longWindow > 0 && len(history) > longWindow {
		history = history[len(history)-longWindow:]
	}

	meanMedianRatio.histories[seriesKey] = history

	shortCount := min(shortWindow, len(history))
	values := ObservationValues(history)
	shortSlice := values[len(values)-shortCount:]
	shortMean := stat.Mean(shortSlice, nil)

	longSlice := values

	if len(values) > shortCount {
		longSlice = values[:len(values)-shortCount]
	}

	longMedian, ok := MedianOf(longSlice)
	transform := datura.Peek[string](meanMedianRatio.artifact, stageKey, "transform")

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

	state.MergeOutput(outputKey, ratio)

	declineOutput := datura.Peek[string](meanMedianRatio.artifact, stageKey, "decline", "output")

	if declineOutput != "" {
		state.MergeOutput(declineOutput, meanMedianRatio.declines[seriesKey+"/"+declineOutput])
	}

	features.Restore(state)

	return state.Read(payload)
}

func (meanMedianRatio *MeanMedianRatio) stageKey() string {
	order := datura.Peek[[]string](meanMedianRatio.artifact, "order")
	stageIndex := int(datura.Peek[float64](meanMedianRatio.artifact, "stageIndex"))

	if stageIndex < 0 {
		stageIndex = 0
	}

	if len(order) > stageIndex {
		return order[stageIndex]
	}

	return ""
}

func (meanMedianRatio *MeanMedianRatio) sample(
	artifact *datura.Artifact,
	sourceKey string,
) (float64, error) {
	if sourceKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"mean-median-ratio: input key required",
			nil,
		))
	}

	sample, err := FeatureColumn(artifact, sourceKey)

	if err == nil {
		if math.IsNaN(sample) || math.IsInf(sample, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"mean-median-ratio: sample is NaN or Inf",
				nil,
			))
		}

		return sample, nil
	}

	rootKey := datura.Peek[string](artifact, "root")
	channelKeys := datura.Peek[[]string](artifact, "inputs")

	if rootKey == "" || len(channelKeys) == 0 {
		return 0, err
	}

	for index, channelKey := range channelKeys {
		if channelKey != sourceKey {
			continue
		}

		sample = datura.Peek[float64](artifact, rootKey, index)

		if math.IsNaN(sample) || math.IsInf(sample, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"mean-median-ratio: sample is NaN or Inf",
				nil,
			))
		}

		return sample, nil
	}

	return 0, err
}

func (meanMedianRatio *MeanMedianRatio) applyTransform(
	seriesKey string,
	stageKey string,
	sample float64,
) float64 {
	transform := datura.Peek[string](meanMedianRatio.artifact, stageKey, "transform")

	if transform == "" {
		return sample
	}

	previousSample := meanMedianRatio.previousSamples[seriesKey]

	meanMedianRatio.previousSamples[seriesKey] = sample

	switch transform {
	case "delta":
		if previousSample <= 0 {
			return sample
		}

		return sample - previousSample
	case "deltaPositive":
		if previousSample <= 0 {
			return sample
		}

		delta := sample - previousSample

		if delta < 0 {
			return 0
		}

		return delta
	default:
		return sample
	}
}

func (meanMedianRatio *MeanMedianRatio) trackDecline(
	seriesKey string,
	stageKey string,
	sample float64,
) {
	declineOutput := datura.Peek[string](meanMedianRatio.artifact, stageKey, "decline", "output")

	if declineOutput == "" {
		return
	}

	previousDelta := meanMedianRatio.previousDeltas[seriesKey]
	decline := 0.0

	if previousDelta > 0 && sample < previousDelta {
		decline = (previousDelta - sample) / previousDelta
	}

	meanMedianRatio.previousDeltas[seriesKey] = sample
	meanMedianRatio.declines[seriesKey+"/"+declineOutput] = decline
}

func (meanMedianRatio *MeanMedianRatio) Close() error {
	return nil
}

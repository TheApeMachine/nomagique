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
	histories       map[string][]float64
	previousSamples map[string]float64
	previousDeltas  map[string]float64
	declines        map[string]float64
}

/*
NewMeanMedianRatio returns a mean-over-median ratio stage wired from config attributes on the artifact.
*/
func NewMeanMedianRatio(artifact *datura.Artifact) *MeanMedianRatio {
	artifact.Inspect("statistic", "mean-median-ratio", "NewMeanMedianRatio()")

	return &MeanMedianRatio{
		artifact:        artifact,
		histories:       map[string][]float64{},
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
	state.Inspect("statistic", "mean-median-ratio", "Read()", "p")

	if _, err := state.Write(meanMedianRatio.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	features := SnapshotFeatures(state)
	stageKey := meanMedianRatio.stageKey()

	if stageKey == "" {
		return state.Read(payload)
	}

	sourceKey := datura.Peek[string](meanMedianRatio.artifact, "inputs", stageKey, "input")
	sample := meanMedianRatio.sample(state, sourceKey)

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return state.Read(payload)
	}

	sample = meanMedianRatio.applyTransform(stageKey, sample)
	meanMedianRatio.trackDecline(stageKey, sample)

	shortHint := int(datura.Peek[float64](meanMedianRatio.artifact, "inputs", stageKey, "shortWindow"))
	longHint := int(datura.Peek[float64](meanMedianRatio.artifact, "inputs", stageKey, "longWindow"))
	outputKey := datura.Peek[string](meanMedianRatio.artifact, "inputs", stageKey, "outputKey")

	if outputKey == "" {
		return state.Read(payload)
	}

	history := meanMedianRatio.histories[stageKey]
	history = append(history, sample)

	shortWindow, longWindow := RollingWindows(history, shortHint, longHint)

	if longWindow > 0 && len(history) > longWindow {
		history = history[len(history)-longWindow:]
	}

	meanMedianRatio.histories[stageKey] = history

	ratio := 0.0

	if longWindow > 0 && len(history) >= longWindow {
		shortCount := min(shortWindow, len(history))

		shortSlice := history[len(history)-shortCount:]
		shortMean := stat.Mean(shortSlice, nil)
		longMedian := MedianOf(history)

		baseline := longMedian

		if baseline <= 0 || baseline < math.Abs(shortMean)/1e6 {
			baseline = math.Abs(shortMean)
		}

		if baseline > 0 {
			ratio = shortMean / baseline
		}
	}

	state.MergeOutput(outputKey, ratio)

	declineOutput := datura.Peek[string](meanMedianRatio.artifact, "inputs", stageKey, "decline", "output")

	if declineOutput != "" {
		if decline, ok := meanMedianRatio.declines[declineOutput]; ok {
			state.MergeOutput(declineOutput, decline)
		}
	}

	features.Restore(state)
	state.Merge("root", "output")

	return state.Read(payload)
}

func (meanMedianRatio *MeanMedianRatio) stageKey() string {
	order := datura.Peek[[]string](meanMedianRatio.artifact, "order")
	stageIndex := int(datura.Peek[float64](meanMedianRatio.artifact, "inputs", "meanMedianRatio", "stageIndex"))

	if stageIndex < 0 {
		stageIndex = int(datura.Peek[float64](meanMedianRatio.artifact, "stageIndex"))
	}

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
) float64 {
	if sourceKey == "" {
		return 0
	}

	sample := FeatureColumn(artifact, sourceKey)

	if sample != 0 || len(datura.Peek[[]float64](artifact, "features")) > 0 {
		if math.IsNaN(sample) || math.IsInf(sample, 0) {
			errnie.Error(errnie.Err(
				errnie.Validation,
				"mean-median-ratio: sample is NaN or Inf",
				nil,
			))
		}

		return sample
	}

	rootKey := datura.Peek[string](artifact, "root")
	channelKeys := datura.Peek[[]string](artifact, "inputs")

	if rootKey == "" || len(channelKeys) == 0 {
		return 0
	}

	for index, channelKey := range channelKeys {
		if channelKey != sourceKey {
			continue
		}

		sample = datura.Peek[float64](artifact, rootKey, index)

		if math.IsNaN(sample) || math.IsInf(sample, 0) {
			errnie.Error(errnie.Err(
				errnie.Validation,
				"mean-median-ratio: sample is NaN or Inf",
				nil,
			))
		}

		return sample
	}

	return 0
}

func (meanMedianRatio *MeanMedianRatio) applyTransform(stageKey string, sample float64) float64 {
	transform := datura.Peek[string](meanMedianRatio.artifact, "inputs", stageKey, "transform")

	if transform == "" {
		return sample
	}

	previousSample := meanMedianRatio.previousSamples[stageKey]

	meanMedianRatio.previousSamples[stageKey] = sample

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

func (meanMedianRatio *MeanMedianRatio) trackDecline(stageKey string, sample float64) {
	declineOutput := datura.Peek[string](meanMedianRatio.artifact, "inputs", stageKey, "decline", "output")

	if declineOutput == "" {
		return
	}

	previousDelta := meanMedianRatio.previousDeltas[stageKey]
	decline := 0.0

	if previousDelta > 0 && sample < previousDelta {
		decline = (previousDelta - sample) / previousDelta
	}

	meanMedianRatio.previousDeltas[stageKey] = sample
	meanMedianRatio.declines[declineOutput] = decline
}

func (meanMedianRatio *MeanMedianRatio) Close() error {
	return nil
}

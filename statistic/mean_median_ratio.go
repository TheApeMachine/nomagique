package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
MeanMedianRatio compares a short-window mean to a long-window median on streamed samples.
*/
type MeanMedianRatio struct {
	config *datura.Artifact
	staged *datura.Artifact
}

/*
NewMeanMedianRatio returns a mean-over-median ratio stage configured on the artifact.
*/
func NewMeanMedianRatio(config *datura.Artifact) *MeanMedianRatio {
	return &MeanMedianRatio{
		config: config,
		staged: datura.Acquire("mean-median-ratio", datura.APPJSON),
	}
}

func (meanMedianRatio *MeanMedianRatio) Read(payload []byte) (int, error) {
	stageKey := meanMedianRatio.stageKey()

	if stageKey == "" {
		return meanMedianRatio.staged.Read(payload)
	}

	rootKey := datura.Peek[string](meanMedianRatio.staged, "root")
	channelKeys := datura.Peek[[]string](meanMedianRatio.staged, "inputs")
	sourceKey := datura.Peek[string](meanMedianRatio.config, "inputs", stageKey, "input")

	sample := meanMedianRatio.sample(
		meanMedianRatio.staged, rootKey, channelKeys, sourceKey,
	)

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return meanMedianRatio.staged.Read(payload)
	}

	if datura.Peek[float64](meanMedianRatio.config, "inputs", stageKey, "useDelta") > 0 {
		sample = meanMedianRatio.deltaSample(sample)
	}

	shortWindow := int(datura.Peek[float64](meanMedianRatio.config, "inputs", stageKey, "shortWindow"))
	longWindow := int(datura.Peek[float64](meanMedianRatio.config, "inputs", stageKey, "longWindow"))
	outputKey := datura.Peek[string](meanMedianRatio.config, "inputs", stageKey, "outputKey")

	if shortWindow <= 0 || longWindow <= 0 || outputKey == "" {
		return meanMedianRatio.staged.Read(payload)
	}

	history := datura.Peek[[]float64](meanMedianRatio.staged, "history")
	history = append(history, sample)

	if len(history) > longWindow {
		history = history[len(history)-longWindow:]
	}

	meanMedianRatio.staged.Poke(history, "history")

	ratio := 0.0

	if len(history) >= longWindow {
		shortCount := shortWindow

		if shortCount > len(history) {
			shortCount = len(history)
		}

		shortSlice := history[len(history)-shortCount:]
		shortMean := stat.Mean(shortSlice, nil)
		longMedian := MedianOf(history)
		denominator := longMedian

		if denominator <= 0 {
			denominator = math.Sqrt(math.Nextafter(1, 2) - 1)
		}

		ratio = shortMean / denominator
	}

	meanMedianRatio.staged.Poke(ratio, "output", outputKey)

	return meanMedianRatio.staged.Read(payload)
}

func (meanMedianRatio *MeanMedianRatio) Write(payload []byte) (int, error) {
	return meanMedianRatio.staged.Write(payload)
}

func (meanMedianRatio *MeanMedianRatio) stageKey() string {
	order := datura.Peek[[]string](meanMedianRatio.config, "order")

	if len(order) == 0 {
		return ""
	}

	return order[0]
}

func (meanMedianRatio *MeanMedianRatio) sample(
	artifact *datura.Artifact,
	rootKey string,
	channelKeys []string,
	sourceKey string,
) float64 {
	if rootKey == "" || sourceKey == "" || len(channelKeys) == 0 {
		return 0
	}

	for index, channelKey := range channelKeys {
		if channelKey != sourceKey {
			continue
		}

		sample := datura.Peek[float64](artifact, rootKey, index)

		if math.IsNaN(sample) || math.IsInf(sample, 0) {
			errnie.Error(errnie.Err(
				errnie.Validation,
				"mean-median-ratio: sample is NaN or Inf",
				nil,
			))

			return sample
		}

		return sample
	}

	return 0
}

func (meanMedianRatio *MeanMedianRatio) deltaSample(sample float64) float64 {
	previous := datura.Peek[float64](meanMedianRatio.staged, "state", "previousSample")
	delta := sample

	if previous > 0 {
		delta = sample - previous

		if delta < 0 {
			delta = 0
		}
	}

	meanMedianRatio.staged.Poke(sample, "state", "previousSample")

	return delta
}

func (meanMedianRatio *MeanMedianRatio) Close() error {
	return nil
}

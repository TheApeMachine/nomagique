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
	bytes  []byte
}

/*
NewMeanMedianRatio returns a mean-over-median ratio stage configured on the artifact.
The config artifact also retains the rolling history across frames; the inbound
artifact (buffered on the config payload) is the per-frame compute state.
*/
func NewMeanMedianRatio(config *datura.Artifact) *MeanMedianRatio {
	config.Inspect("statistic", "mean-median-ratio", "NewMeanMedianRatio()")

	return &MeanMedianRatio{
		config: config,
	}
}

func (meanMedianRatio *MeanMedianRatio) Read(payload []byte) (int, error) {
	state := datura.Acquire("mean-median-ratio-state", datura.APPJSON)
	state.Inspect("statistic", "mean-median-ratio", "Read()", "p")

	if _, err := state.Write(meanMedianRatio.bytes); err != nil {
		state.Release()

		return 0, err
	}

	defer state.Release()

	stageKey := meanMedianRatio.stageKey()

	if stageKey == "" {
		return state.Read(payload)
	}

	rootKey := datura.Peek[string](state, "root")
	channelKeys := datura.Peek[[]string](state, "inputs")
	sourceKey := datura.Peek[string](meanMedianRatio.config, "inputs", stageKey, "input")

	sample := meanMedianRatio.sample(state, rootKey, channelKeys, sourceKey)

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return state.Read(payload)
	}

	if datura.Peek[float64](meanMedianRatio.config, "inputs", stageKey, "useDelta") > 0 {
		sample = meanMedianRatio.deltaSample(sample)

		previousDelta := datura.Peek[float64](meanMedianRatio.config, "previousDelta")
		liftDecline := 0.0

		if previousDelta > 0 && sample < previousDelta {
			liftDecline = (previousDelta - sample) / previousDelta
		}

		meanMedianRatio.config.Merge("previousDelta", sample)
		meanMedianRatio.config.Merge("rvolDecline", liftDecline)
	}

	shortHint := int(datura.Peek[float64](meanMedianRatio.config, "inputs", stageKey, "shortWindow"))
	longHint := int(datura.Peek[float64](meanMedianRatio.config, "inputs", stageKey, "longWindow"))
	outputKey := datura.Peek[string](meanMedianRatio.config, "inputs", stageKey, "outputKey")

	if outputKey == "" {
		return state.Read(payload)
	}

	// Rolling history lives on the config artifact so it survives across frames;
	// the per-frame state buffer is replaced every Write.
	history := datura.Peek[[]float64](meanMedianRatio.config, "history")
	history = append(history, sample)

	shortWindow, longWindow := RollingWindows(history, shortHint, longHint)

	if longWindow > 0 && len(history) > longWindow {
		history = history[len(history)-longWindow:]
	}

	meanMedianRatio.config.Merge("history", history)

	ratio := 0.0

	if longWindow > 0 && len(history) >= longWindow {
		shortCount := shortWindow

		if shortCount > len(history) {
			shortCount = len(history)
		}

		shortSlice := history[len(history)-shortCount:]
		shortMean := stat.Mean(shortSlice, nil)
		longMedian := MedianOf(history)

		// A non-positive baseline makes the relative ratio undefined; dividing by
		// an epsilon floor would explode RVOL to ~1e8+. Treat it as no measurable
		// lift (neutral) rather than a spurious spike.
		// Use the median as the baseline when it is a meaningful fraction of the
		// short-window mean; otherwise fall back to the mean magnitude so a
		// near-zero median cannot explode the ratio (epsilon division) nor zero
		// it out. This keeps RVOL ~1 at baseline and proportional under spikes.
		baseline := longMedian

		if baseline <= 0 || baseline < math.Abs(shortMean)/1e6 {
			baseline = math.Abs(shortMean)
		}

		if baseline > 0 {
			ratio = shortMean / baseline
		}
	}

	meanMedianRatio.config.Merge("previousRatio", ratio)

	output := datura.Acquire("mean-median-ratio-output", datura.APPJSON)
	output.WithPayload(state.DecryptPayload())
	output.MergeOutput(outputKey, ratio)

	output.Inspect("statistic", "mean-median-ratio", "Read()", "output")

	return output.Read(payload)
}

func (meanMedianRatio *MeanMedianRatio) Write(payload []byte) (int, error) {
	meanMedianRatio.bytes = append(meanMedianRatio.bytes[:0], payload...)

	return len(payload), nil
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

		// Features flow as a positional []float64 under rootKey, aligned with the
		// inputs order; resolve the channel name to its index to read the value.
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
	previous := datura.Peek[float64](meanMedianRatio.config, "previousSample")
	delta := sample

	if previous > 0 {
		delta = sample - previous

		if delta < 0 {
			delta = 0
		}
	}

	meanMedianRatio.config.Merge("previousSample", sample)

	return delta
}

func (meanMedianRatio *MeanMedianRatio) Close() error {
	return nil
}

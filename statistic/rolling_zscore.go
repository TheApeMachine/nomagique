package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
RollingZScore normalizes the current sample against its retained series on the stage instance.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type RollingZScore struct {
	artifact *datura.Artifact
	samples  []float64
}

/*
NewRollingZScore returns a rolling z-score stage wired from config attributes on the artifact.
*/
func NewRollingZScore(artifact *datura.Artifact) *RollingZScore {
	artifact.Inspect("statistic", "rolling-zscore", "NewRollingZScore()")

	return &RollingZScore{
		artifact: artifact,
		samples:  []float64{},
	}
}

func (rollingZScore *RollingZScore) Write(payload []byte) (int, error) {
	rollingZScore.artifact.WithPayload(payload)
	return len(payload), nil
}

func (rollingZScore *RollingZScore) Read(payload []byte) (int, error) {
	state := datura.Acquire("rolling-zscore-state", datura.APPJSON)
	state.Inspect("statistic", "rolling-zscore", "Read()", "p")

	if _, err := state.Write(rollingZScore.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	features := SnapshotFeatures(state)
	sample := datura.Peek[float64](state, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"rolling-zscore: sample is non-finite",
			nil,
		))
	}

	prior := rollingZScore.samples

	if len(prior) < 2 {
		samples := append(prior, sample)
		stageKey := rollingZScore.stageKey()
		longHint := 0

		if stageKey != "" {
			longHint = int(datura.Peek[float64](rollingZScore.artifact, stageKey, "longWindow"))
		}

		_, longWindow := RollingWindows(samples, 0, longHint)

		if longWindow > 0 && len(samples) > longWindow {
			samples = samples[len(samples)-longWindow:]
		}

		rollingZScore.samples = samples

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"rolling-zscore: insufficient prior samples",
			nil,
		))
	}

	meanSample := stat.Mean(prior, nil)
	stdSample := stat.StdDev(prior, nil)

	if stdSample <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"rolling-zscore: prior standard deviation is zero",
			nil,
		))
	}

	score := (sample - meanSample) / stdSample

	samples := append(prior, sample)
	stageKey := rollingZScore.stageKey()
	longHint := 0

	if stageKey != "" {
		longHint = int(datura.Peek[float64](rollingZScore.artifact, stageKey, "longWindow"))
	}

	_, longWindow := RollingWindows(samples, 0, longHint)

	if longWindow > 0 && len(samples) > longWindow {
		samples = samples[len(samples)-longWindow:]
	}

	rollingZScore.samples = samples

	outputKey := "value"

	if stageKey != "" {
		configuredKey := datura.Peek[string](rollingZScore.artifact, stageKey, "outputKey")

		if configuredKey != "" {
			outputKey = configuredKey
		}
	}

	state.MergeOutput(outputKey, score)
	features.Restore(state)
	state.Merge("root", "output")

	return state.Read(payload)
}

func (rollingZScore *RollingZScore) stageKey() string {
	stageKey := datura.Peek[string](rollingZScore.artifact, "stage")

	if stageKey != "" {
		return stageKey
	}

	order := datura.Peek[[]string](rollingZScore.artifact, "order")
	stageIndex := int(datura.Peek[float64](rollingZScore.artifact, "precursor", "stageIndex"))

	if stageIndex <= 0 {
		stageIndex = int(datura.Peek[float64](rollingZScore.artifact, "stageIndex"))
	}

	if stageIndex >= 0 && len(order) > stageIndex {
		return order[stageIndex]
	}

	return ""
}

func (rollingZScore *RollingZScore) Close() error {
	return nil
}

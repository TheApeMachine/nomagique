package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/statistic"
)

/*
LogReturn computes a lagged log return from a retained sample series on the stage instance.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type LogReturn struct {
	artifact *datura.Artifact
}

/*
NewLogReturn returns a log-return stage wired from config attributes on the artifact.
*/
func NewLogReturn(artifact *datura.Artifact) *LogReturn {
	artifact.Inspect("adaptive", "log-return", "NewLogReturn()")

	return &LogReturn{
		artifact: artifact,
	}
}

func (logReturn *LogReturn) Write(payload []byte) (int, error) {
	logReturn.artifact.WithPayload(payload)
	return len(payload), nil
}

func (logReturn *LogReturn) Read(payload []byte) (int, error) {
	state := datura.Acquire("log-return-state", datura.APPJSON)
	state.Inspect("adaptive", "log-return", "Read()", "p")

	if _, err := state.Write(logReturn.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	features := statistic.SnapshotFeatures(state)
	stageKey := logReturn.stageKey()

	if stageKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"log-return: stage config required",
			nil,
		))
	}

	inputKey := datura.Peek[string](logReturn.artifact, stageKey, "input")
	outputKey := datura.Peek[string](logReturn.artifact, stageKey, "outputKey")

	if outputKey == "" {
		outputKey = "value"
	}

	if inputKey == "" {
		var wireErr error
		inputKey, wireErr = statistic.WireInputKey(logReturn.artifact, state)

		if wireErr != nil {
			return 0, wireErr
		}
	}

	sample, err := logReturn.resolveSample(state, inputKey)

	if err != nil {
		return 0, err
	}

	if sample <= 0 || math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"log-return: sample is non-positive or non-finite",
			nil,
		))
	}

	longHint := int(datura.Peek[float64](logReturn.artifact, stageKey, "longWindow"))

	returnLag, err := statistic.ReturnLag(
		datura.Peek[[]float64](logReturn.artifact, stageKey, "logSamples"),
		int(datura.Peek[float64](logReturn.artifact, stageKey, "returnLag")),
		longHint,
	)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"log-return: unable to resolve return lag",
			err,
		))
	}

	samples := datura.Peek[[]float64](logReturn.artifact, stageKey, "logSamples")
	samples = append(samples, sample)

	_, longWindow, err := statistic.RollingWindows(samples, 0, longHint)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"log-return: rolling windows failed",
			err,
		))
	}

	if longWindow > 0 && len(samples) > longWindow+returnLag {
		samples = samples[len(samples)-longWindow-returnLag:]
	}

	logReturn.artifact.Poke(samples, stageKey, "logSamples")

	if len(samples) <= returnLag {
		features.Restore(state)

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"log-return: insufficient samples",
			nil,
		))
	}

	anchorSample := samples[len(samples)-returnLag-1]

	if anchorSample <= 0 || math.IsNaN(anchorSample) || math.IsInf(anchorSample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"log-return: anchor sample is non-positive or non-finite",
			nil,
		))
	}

	logReturnValue := math.Log(sample / anchorSample)
	state.MergeOutput(outputKey, logReturnValue)
	features.Restore(state)
	state.Merge("root", "output")

	return state.Read(payload)
}

func (logReturn *LogReturn) resolveSample(
	state *datura.Artifact,
	inputKey string,
) (float64, error) {
	rootKey := datura.Peek[string](state, "root")

	if rootKey == "output" && inputKey != "" {
		if datura.KeyPresent(state, "output", inputKey) {
			return statistic.WireScalarAt(logReturn.artifact, state, "output", inputKey)
		}
	}

	sample, err := statistic.WireScalar(logReturn.artifact, state, inputKey)

	if err != nil && inputKey != "sample" {
		return statistic.WireScalar(logReturn.artifact, state, "sample")
	}

	return sample, err
}

func (logReturn *LogReturn) stageKey() string {
	stageKey := datura.Peek[string](logReturn.artifact, "stage")

	if stageKey != "" {
		return stageKey
	}

	order := datura.Peek[[]string](logReturn.artifact, "order")
	stageIndex := int(datura.Peek[float64](logReturn.artifact, "precursor", "stageIndex"))

	if stageIndex <= 0 {
		stageIndex = int(datura.Peek[float64](logReturn.artifact, "stageIndex"))
	}

	if stageIndex >= 0 && len(order) > stageIndex {
		return order[stageIndex]
	}

	return ""
}

func (logReturn *LogReturn) Close() error {
	return nil
}

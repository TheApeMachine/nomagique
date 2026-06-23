package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
LogReturn computes a lagged log return from a retained sample series on the stage instance.
The constructor artifact holds config; Write buffers inbound payload.
*/
type LogReturn struct {
	artifact  *datura.Artifact
	samples   []float64
	inputKey  string
	outputKey string
	returnLag int
}

/*
NewLogReturn returns a log-return stage wired from config attributes on the artifact.
*/
func NewLogReturn(artifact *datura.Artifact) *LogReturn {
	artifact.Inspect("adaptive", "log-return", "NewLogReturn()")

	stage := &LogReturn{
		artifact: artifact,
	}

	stageKey := datura.Peek[string](artifact, "stage")

	if stageKey == "" {
		order := datura.Peek[[]string](artifact, "order")
		stageIndex := int(datura.Peek[float64](artifact, "precursor", "stageIndex"))

		if stageIndex <= 0 {
			stageIndex = int(datura.Peek[float64](artifact, "stageIndex"))
		}

		if stageIndex >= 0 && len(order) > stageIndex {
			stageKey = order[stageIndex]
		}
	}

	if stageKey != "" {
		stage.inputKey = datura.Peek[string](artifact, stageKey, "input")
		stage.outputKey = datura.Peek[string](artifact, stageKey, "outputKey")
		stage.returnLag = int(datura.Peek[float64](artifact, stageKey, "returnLag"))
	}

	if stage.outputKey == "" {
		stage.outputKey = "value"
	}

	if stage.returnLag <= 0 {
		stage.returnLag = 1
	}

	return stage
}

func (logReturn *LogReturn) Write(p []byte) (int, error) {
	logReturn.artifact.WithPayload(p)
	return len(p), nil
}

func (logReturn *LogReturn) Read(payload []byte) (int, error) {
	state := datura.Acquire("log-return-state", datura.APPJSON)
	state.Inspect("adaptive", "log-return", "Read()", "p")

	if _, err := state.Write(logReturn.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"log-return: state write failed",
			err,
		))
	}

	outputKey := logReturn.outputKey
	inputKey := logReturn.inputKey

	root := datura.Peek[string](state, "root")
	inputs := datura.Peek[[]string](state, "inputs")

	if inputKey == "" && len(inputs) > 0 {
		inputKey = inputs[0]
	}

	if inputKey == "" {
		inputKey = "sample"
	}

	sample := datura.Peek[float64](state, root, inputKey)

	if sample <= 0 || math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"log-return: sample is non-positive or non-finite",
			nil,
		))
	}

	returnLag := logReturn.returnLag

	logReturn.samples = append(logReturn.samples, sample)

	if len(logReturn.samples) <= returnLag {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"log-return: insufficient samples",
			nil,
		))
	}

	anchorSample := logReturn.samples[len(logReturn.samples)-returnLag-1]

	if anchorSample <= 0 || math.IsNaN(anchorSample) || math.IsInf(anchorSample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"log-return: anchor sample is non-positive or non-finite",
			nil,
		))
	}

	logReturnValue := math.Log(sample / anchorSample)

	state.Poke("output", "root")
	state.Poke([]string{outputKey}, "inputs")
	state.MergeOutput(outputKey, logReturnValue)

	return state.Read(payload)
}

func (logReturn *LogReturn) Close() error {
	return nil
}

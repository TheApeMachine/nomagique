package adaptive

import (
	"math"
	"time"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
LogReturn computes a lagged log return from a retained sample series on the stage instance.
*/
type LogReturn struct {
	artifact *datura.Artifact
	samples  map[string][]struct {
		value float64
		at    time.Time
	}
}

/*
NewLogReturn returns a log-return stage wired from config attributes on the artifact.
*/
func NewLogReturn(artifact *datura.Artifact) *LogReturn {
	return &LogReturn{
		artifact: artifact,
		samples: map[string][]struct {
			value float64
			at    time.Time
		}{},
	}
}

func (logReturn *LogReturn) Read(payload []byte) (int, error) {
	state := datura.Acquire("log-return-state", datura.APPJSON)

	if _, err := state.Write(logReturn.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"log-return: state write failed",
			err,
		))
	}


	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"log-return: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"log-return: inputs required",
			nil,
		))
	}

	stageKey := datura.Peek[string](logReturn.artifact, "stage")

	if stageKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"log-return: stage required",
			nil,
		))
	}

	configInput := datura.Peek[string](logReturn.artifact, stageKey, "input")

	if configInput == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"log-return: input required",
			nil,
		))
	}

	outputKey := datura.Peek[string](logReturn.artifact, stageKey, "outputKey")

	if outputKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"log-return: outputKey required",
			nil,
		))
	}

	returnLag := int(datura.Peek[float64](logReturn.artifact, stageKey, "returnLag"))

	if returnLag <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"log-return: returnLag required",
			nil,
		))
	}

	var sample float64
	found := false

	for index, input := range inputs {
		if input != configInput {
			continue
		}

		if rootKey == "features" {
			features := datura.Peek[[]float64](state, rootKey)

			if index >= len(features) {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"log-return: feature index out of range",
					nil,
				))
			}

			sample = features[index]
		}

		if rootKey != "features" {
			sample = datura.Peek[float64](state, rootKey, input)
		}

		found = true
	}

	if !found {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"log-return: input not in inputs",
			nil,
		))
	}

	if sample <= 0 || math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"log-return: sample is non-positive or non-finite",
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
			"log-return: event timestamp required",
			nil,
		))
	}

	observed := time.Unix(0, timestamp)
	history := logReturn.samples[seriesKey]

	if len(history) > 0 && observed.Before(history[len(history)-1].at) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"log-return: event timestamp must not regress",
			nil,
		))
	}

	history = append(history, struct {
		value float64
		at    time.Time
	}{value: sample, at: observed})

	longHint := int(datura.Peek[float64](logReturn.artifact, stageKey, "longWindow"))
	longWindow := len(history)

	if longHint > 0 {
		longWindow = longHint
	}

	keep := longWindow + returnLag

	if keep > 0 && len(history) > keep {
		history = history[len(history)-keep:]
	}

	logReturn.samples[seriesKey] = history

	anchorIndex := len(history) - returnLag - 1

	if anchorIndex < 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"log-return: insufficient history for returnLag",
			nil,
		))
	}

	anchorSample := history[anchorIndex].value

	if anchorSample <= 0 || math.IsNaN(anchorSample) || math.IsInf(anchorSample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"log-return: anchor sample is non-positive or non-finite",
			nil,
		))
	}

	logReturnValue := math.Log(sample / anchorSample)
	state.MergeOutput(outputKey, logReturnValue)
	state.Poke("output", "root")
	state.Poke([]string{outputKey}, "inputs")

	return state.Read(payload)
}

func (logReturn *LogReturn) Write(p []byte) (int, error) {
	logReturn.artifact.WithPayload(p)
	return len(p), nil
}

func (logReturn *LogReturn) Close() error {
	return nil
}

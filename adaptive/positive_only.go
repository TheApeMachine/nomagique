package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
PositiveOnly optionally clamps a sample at zero from below.
*/
type PositiveOnly struct {
	artifact *datura.Artifact
}

/*
NewPositiveOnly returns a positive-only gate wired from config attributes on the artifact.
*/
func NewPositiveOnly(artifact *datura.Artifact) *PositiveOnly {
	return &PositiveOnly{
		artifact: artifact,
	}
}

func (positiveOnly *PositiveOnly) Read(payload []byte) (int, error) {
	state := datura.Acquire("positive-only-state", datura.APPJSON)

	if _, err := state.Unpack(positiveOnly.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"positive-only: state write failed",
			err,
		))
	}

	stageKey := datura.Peek[string](positiveOnly.artifact, "stage")
	outputKey := datura.Peek[string](positiveOnly.artifact, "outputKey")
	positiveOnlyFlag := datura.Peek[float64](positiveOnly.artifact, "positiveOnly")

	if stageKey == "" {
		order := datura.Peek[[]string](positiveOnly.artifact, "order")
		stageIndex := int(datura.Peek[float64](positiveOnly.artifact, "stageIndex"))

		if stageIndex >= 0 && stageIndex < len(order) {
			stageKey = order[stageIndex]
		}
	}

	if stageKey != "" {
		outputKey = datura.Peek[string](positiveOnly.artifact, stageKey, "outputKey")
		positiveOnlyFlag = datura.Peek[float64](positiveOnly.artifact, stageKey, "positiveOnly")
	}

	if outputKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"positive-only: outputKey required",
			nil,
		))
	}

	rootKey := datura.Peek[string](state, "root")
	inputs := datura.Peek[[]string](state, "inputs")

	if rootKey == "" || len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"positive-only: root and inputs required",
			nil,
		))
	}

	var score float64
	found := false

	for _, input := range inputs {
		if input != outputKey {
			continue
		}

		score = datura.Peek[float64](state, rootKey, input)
		found = true
	}

	if !found {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"positive-only: outputKey not in inputs",
			nil,
		))
	}

	if math.IsNaN(score) || math.IsInf(score, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"positive-only: score is non-finite",
			nil,
		))
	}

	if positiveOnlyFlag > 0 {
		score = math.Max(0, score)
	}

	state.MergeOutput(outputKey, score)
	state.Poke("output", "root")
	state.Poke([]string{outputKey}, "inputs")

	return state.PackInto(payload)
}

func (positiveOnly *PositiveOnly) Write(p []byte) (int, error) {
	positiveOnly.artifact.WithPayload(p)
	return len(p), nil
}

func (positiveOnly *PositiveOnly) Close() error {
	return nil
}

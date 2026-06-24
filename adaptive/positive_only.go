package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
PositiveOnly optionally clamps a sample at zero from below.
The constructor artifact holds config; Write buffers inbound payload.
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

	if _, err := state.Write(positiveOnly.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"positive-only: state write failed",
			err,
		))
	}


	stageKey := datura.Peek[string](positiveOnly.artifact, "stage")

	if stageKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"positive-only: stage config required",
			nil,
		))
	}

	outputKey := datura.Peek[string](positiveOnly.artifact, stageKey, "outputKey")

	if outputKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"positive-only: outputKey required",
			nil,
		))
	}

	root := datura.Peek[string](state, "root")
	inputs := datura.Peek[[]string](state, "inputs")

	if root == "" || len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"positive-only: wire required",
			nil,
		))
	}

	var score float64
	found := false

	for _, input := range inputs {
		if input != outputKey {
			continue
		}

		score = datura.Peek[float64](state, root, input)
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

	if datura.Peek[float64](positiveOnly.artifact, stageKey, "positiveOnly") > 0 {
		score = math.Max(0, score)
	}

	state.MergeOutput(outputKey, score)
	state.Poke("output", "root")
	state.Poke([]string{outputKey}, "inputs")

	return state.Read(payload)
}

func (positiveOnly *PositiveOnly) Write(p []byte) (int, error) {
	positiveOnly.artifact.WithPayload(p)
	return len(p), nil
}

func (positiveOnly *PositiveOnly) Close() error {
	return nil
}

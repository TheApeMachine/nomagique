package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
PositiveOnly optionally clamps a sample at zero from below.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type PositiveOnly struct {
	artifact *datura.Artifact
}

/*
NewPositiveOnly returns a positive-only gate wired from config attributes on the artifact.
*/
func NewPositiveOnly(artifact *datura.Artifact) *PositiveOnly {
	artifact.Inspect("adaptive", "positive-only", "NewPositiveOnly()")

	return &PositiveOnly{
		artifact: artifact,
	}
}

func (positiveOnly *PositiveOnly) Write(payload []byte) (int, error) {
	positiveOnly.artifact.WithPayload(payload)
	return len(payload), nil
}

func (positiveOnly *PositiveOnly) Read(payload []byte) (int, error) {
	state := datura.Acquire("positive-only-state", datura.APPJSON)
	state.Inspect("adaptive", "positive-only", "Read()", "p")

	if _, err := state.Write(positiveOnly.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	stageKey := datura.Peek[string](positiveOnly.artifact, "stage")

	if stageKey == "" {
		order := datura.Peek[[]string](positiveOnly.artifact, "order")
		stageIndex := int(datura.Peek[float64](positiveOnly.artifact, "inputs", "precursor", "stageIndex"))

		if stageIndex <= 0 {
			stageIndex = int(datura.Peek[float64](positiveOnly.artifact, "stageIndex"))
		}

		if stageIndex >= 0 && len(order) > stageIndex {
			stageKey = order[stageIndex]
		}
	}

	if stageKey == "" {
		return state.Read(payload)
	}

	outputKey := datura.Peek[string](positiveOnly.artifact, "inputs", stageKey, "outputKey")

	if outputKey == "" {
		return state.Read(payload)
	}

	score := datura.Peek[float64](state, "sample")

	if datura.Peek[float64](positiveOnly.artifact, "inputs", stageKey, "positiveOnly") > 0 {
		score = math.Max(0, score)
	}

	state.MergeOutput(outputKey, score)
	state.Merge("root", "output")
	state.Merge("inputs", []string{outputKey})
	return state.Read(payload)
}

func (positiveOnly *PositiveOnly) Close() error {
	return nil
}

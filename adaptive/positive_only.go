package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/statistic"
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

	features := statistic.SnapshotFeatures(state)
	stageKey := positiveOnly.stageKey()

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

	score := positiveOnly.score(state, outputKey)

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
	features.Restore(state)
	state.Merge("root", "output")

	return state.Read(payload)
}

func (positiveOnly *PositiveOnly) stageKey() string {
	stageKey := datura.Peek[string](positiveOnly.artifact, "stage")

	if stageKey != "" {
		return stageKey
	}

	order := datura.Peek[[]string](positiveOnly.artifact, "order")
	stageIndex := int(datura.Peek[float64](positiveOnly.artifact, "precursor", "stageIndex"))

	if stageIndex <= 0 {
		stageIndex = int(datura.Peek[float64](positiveOnly.artifact, "stageIndex"))
	}

	if stageIndex >= 0 && len(order) > stageIndex {
		return order[stageIndex]
	}

	return ""
}

func (positiveOnly *PositiveOnly) score(state *datura.Artifact, outputKey string) float64 {
	rootKey := datura.Peek[string](state, "root")

	switch rootKey {
	case "output":
		return datura.Peek[float64](state, "output", outputKey)
	case "sample":
		return datura.Peek[float64](state, "sample")
	default:
		return datura.Peek[float64](state, "sample")
	}
}

func (positiveOnly *PositiveOnly) Close() error {
	return nil
}

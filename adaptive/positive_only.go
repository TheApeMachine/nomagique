package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
PositiveOnly optionally clamps a sample at zero from below.
*/
type PositiveOnly struct {
	config *datura.Artifact
	bytes  []byte
}

/*
NewPositiveOnly returns a positive-only gate configured on the artifact.
*/
func NewPositiveOnly(config *datura.Artifact) *PositiveOnly {
	return &PositiveOnly{
		config: config,
	}
}

func (positiveOnly *PositiveOnly) Write(payload []byte) (int, error) {
	positiveOnly.bytes = append(positiveOnly.bytes[:0], payload...)

	return len(payload), nil
}

func (positiveOnly *PositiveOnly) Read(payload []byte) (int, error) {
	state := datura.Acquire("positive-only-state", datura.APPJSON)

	if _, err := state.Write(positiveOnly.bytes); err != nil {
		state.Release()

		return 0, err
	}

	defer state.Release()

	stageKey := datura.Peek[string](positiveOnly.config, "stage")

	if stageKey == "" {
		order := datura.Peek[[]string](positiveOnly.config, "order")
		stageIndex := int(datura.Peek[float64](positiveOnly.config, "inputs", "precursor", "stageIndex"))

		if stageIndex <= 0 {
			stageIndex = int(datura.Peek[float64](positiveOnly.config, "stageIndex"))
		}

		if stageIndex >= 0 && len(order) > stageIndex {
			stageKey = order[stageIndex]
		}
	}

	if stageKey == "" {
		return state.Read(payload)
	}

	outputKey := datura.Peek[string](positiveOnly.config, "inputs", stageKey, "outputKey")

	if outputKey == "" {
		return state.Read(payload)
	}

	score := datura.Peek[float64](state, "sample")

	if datura.Peek[float64](positiveOnly.config, "inputs", stageKey, "positiveOnly") > 0 {
		score = math.Max(0, score)
	}

	output := datura.Acquire("positive-only-output", datura.APPJSON)
	output.WithPayload(state.DecryptPayload())
	output.MergeOutput(outputKey, score)

	return output.Read(payload)
}

func (positiveOnly *PositiveOnly) Close() error {
	return nil
}

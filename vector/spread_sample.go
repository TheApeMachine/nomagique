package vector

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/statistic"
)

/*
SpreadSample derives a relative spread sample from two feature slots.
*/
type SpreadSample struct {
	config *datura.Artifact
}

/*
NewSpreadSample returns a spread sample stage configured on the artifact.
*/
func NewSpreadSample(config *datura.Artifact) *SpreadSample {
	config.Inspect("vector", "spread-sample", "NewSpreadSample()")

	return &SpreadSample{
		config: config,
	}
}

func (spreadSample *SpreadSample) Write(p []byte) (int, error) {
	spreadSample.config.WithPayload(p)
	return len(p), nil
}

func (spreadSample *SpreadSample) Read(p []byte) (int, error) {
	state := datura.Acquire("spread-sample-state", datura.APPJSON)
	state.Inspect("vector", "spread-sample", "Read()", "p")

	if _, err := state.Write(spreadSample.config.DecryptPayload()); err != nil {
		state.Release()
		return 0, err
	}

	defer state.Release()

	features := statistic.SnapshotFeatures(state)
	sourceKeys := datura.Peek[[]string](spreadSample.config, "spread", "inputs")

	if len(sourceKeys) < 2 {
		return state.Read(p)
	}

	left := statistic.FeatureColumn(state, sourceKeys[0])
	right := statistic.FeatureColumn(state, sourceKeys[1])
	mid := (left + right) / 2
	spread := 0.0

	if mid > 0 {
		spread = math.Abs(right-left) / mid
	}

	features.Restore(state)
	state.MergeOutput("spread", spread)
	state.Merge("sample", spread)

	return state.Read(p)
}

func (spreadSample *SpreadSample) Close() error {
	return nil
}

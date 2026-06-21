package vector

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
SpreadSample derives a relative spread sample from two feature slots.
*/
type SpreadSample struct {
	config *datura.Artifact
	bytes  []byte
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
	spreadSample.bytes = append(spreadSample.bytes[:0], p...)

	return len(p), nil
}

func (spreadSample *SpreadSample) Read(p []byte) (int, error) {
	state := datura.Acquire("spread-sample-state", datura.APPJSON)
	state.Inspect("vector", "spread-sample", "Read()", "p")

	if _, err := state.Write(spreadSample.bytes); err != nil {
		state.Release()

		return 0, err
	}

	defer state.Release()

	rootKey := datura.Peek[string](state, "root")
	channelKeys := datura.Peek[[]string](state, "inputs")
	sourceKeys := datura.Peek[[]string](spreadSample.config, "inputs", "spread", "inputs")

	if rootKey == "" || len(channelKeys) == 0 || len(sourceKeys) < 2 {
		return state.Read(p)
	}

	left := spreadSample.sample(state, rootKey, channelKeys, sourceKeys[0])
	right := spreadSample.sample(state, rootKey, channelKeys, sourceKeys[1])
	mid := (left + right) / 2
	spread := 0.0

	if mid > 0 {
		spread = math.Abs(right-left) / mid
	}

	output := datura.Acquire("spread-sample-output", datura.APPJSON)
	output.WithPayload(state.DecryptPayload())
	output.Merge("sample", spread)

	output.Inspect("vector", "spread-sample", "Read()", "output")

	return output.Read(p)
}

func (spreadSample *SpreadSample) sample(
	artifact *datura.Artifact,
	rootKey string,
	channelKeys []string,
	sourceKey string,
) float64 {
	if rootKey == "" || sourceKey == "" || len(channelKeys) == 0 {
		return 0
	}

	for index, channelKey := range channelKeys {
		if channelKey != sourceKey {
			continue
		}

		return datura.Peek[float64](artifact, rootKey, index)
	}

	return 0
}

func (spreadSample *SpreadSample) Close() error {
	return nil
}

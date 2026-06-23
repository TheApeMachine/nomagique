package vector

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
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

	if _, err := state.Write(spreadSample.config.DecryptPayload()); err != nil {
		state.Release()

		return 0, err
	}

	state.Inspect("vector", "spread-sample", "Read()", "p")

	defer state.Release()

	inputs := datura.Peek[[]string](spreadSample.config, "spread", "inputs")

	if len(inputs) == 0 {
		inputs = datura.Peek[[]string](state, "inputs")
	}

	low := math.Inf(1)
	high := math.Inf(-1)

	for _, input := range inputs {
		data, dataErr := statistic.FeatureColumn(state, input)

		if dataErr != nil {
			root := datura.Peek[string](state, "root")
			data = datura.Peek[float64](state, root, input)
		}

		if math.IsNaN(data) || math.IsInf(data, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"spread-sample: input is non-finite",
				nil,
			))
		}

		low = math.Min(low, data)
		high = math.Max(high, data)
	}

	mid := (low + high) / 2

	if mid <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"spread-sample: mid price must be positive",
			nil,
		))
	}

	value := (high - low) / mid

	outputKey := datura.Peek[string](spreadSample.config, "spread", "outputKey")

	if outputKey == "" {
		outputKey = "spread"
	}

	state.MergeOutput(outputKey, value)
	state.Poke("output", "root")
	state.Poke([]string{outputKey}, "inputs")

	return state.Read(p)
}

func (spreadSample *SpreadSample) Close() error {
	return nil
}

package vector

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
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

func (spreadSample *SpreadSample) Read(p []byte) (int, error) {
	state := datura.Acquire("spread-sample-state", datura.APPJSON)

	if _, err := state.Write(spreadSample.config.DecryptPayload()); err != nil {
		state.Release()

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"spread-sample: state write failed",
			err,
		))
	}

	defer state.Release()

	spreadInputs := datura.Peek[[]string](spreadSample.config, "spread", "inputs")

	if len(spreadInputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"spread-sample: spread.inputs required",
			nil,
		))
	}

	outputKey := datura.Peek[string](spreadSample.config, "spread", "outputKey")

	if outputKey == "" {
		outputKey = "spread"
	}

	low := math.Inf(1)
	high := math.Inf(-1)

	for _, input := range spreadInputs {
		data, err := spreadInputValue(state, input)

		if err != nil {
			return 0, err
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

	state.MergeOutput(outputKey, value)
	state.Poke("output", "root")

	inputs := datura.Peek[[]string](state, "inputs")

	if inputs == nil {
		inputs = []string{}
	}

	found := false

	for _, input := range inputs {
		if input == outputKey {
			found = true
		}
	}

	if !found {
		inputs = append(inputs, outputKey)
	}

	state.Poke(inputs, "inputs")

	return state.Read(p)
}

func (spreadSample *SpreadSample) Write(p []byte) (int, error) {
	spreadSample.config.WithPayload(p)
	return len(p), nil
}

func (spreadSample *SpreadSample) Close() error {
	return nil
}

func spreadInputValue(state *datura.Artifact, inputKey string) (float64, error) {
	rootKey := datura.Peek[string](state, "root")
	wireInputs := datura.Peek[[]string](state, "inputs")

	if rootKey != "" && len(wireInputs) > 0 {
		for wireIndex, wireInput := range wireInputs {
			if wireInput != inputKey {
				continue
			}

			if rootKey == "features" {
				featureSlice := datura.Peek[[]float64](state, rootKey)

				if wireIndex >= len(featureSlice) {
					return 0, errnie.Error(errnie.Err(
						errnie.Validation,
						"spread-sample: feature index out of range",
						nil,
					))
				}

				return featureSlice[wireIndex], nil
			}

			return datura.Peek[float64](state, rootKey, wireInput), nil
		}
	}

	return 0, errnie.Error(errnie.Err(
		errnie.Validation,
		"spread-sample: spread input not in inputs",
		nil,
	))
}

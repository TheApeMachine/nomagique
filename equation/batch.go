package equation

import (
	"io"
	"sort"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Features returns the feature vector staged on the merged artifact payload.
*/
func Features(artifact *datura.Artifact) []float64 {
	return datura.Peek[[]float64](artifact, "features")
}

func stageState(bytes []byte) (*datura.Artifact, error) {
	state := datura.Acquire("equation-state", datura.APPJSON)

	if len(bytes) == 0 {
		state.Release()

		return nil, io.EOF
	}

	if _, err := state.Unpack(bytes); err != nil {
		state.Release()

		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"equation: stage state write failed",
			err,
		))
	}

	return state, nil
}

func rejectStage(state *datura.Artifact, message string) (int, error) {
	state.Release()

	return 0, errnie.Error(errnie.Err(errnie.Validation, message, nil))
}

func emitOutput(state *datura.Artifact, payload []byte, fields datura.Map[float64]) (int, error) {
	defer state.Release()

	outputInputs := make([]string, 0, len(fields)+1)
	output := make(map[string]any, len(fields)+1)

	for key, value := range fields {
		output[key] = value
		outputInputs = append(outputInputs, key)
	}

	sort.Strings(outputInputs)

	if _, hasStrength := fields["strength"]; !hasStrength {
		if value, ok := fields["value"]; ok {
			output["strength"] = value
			outputInputs = append(outputInputs, "strength")
		}
	}

	state.MergeOutputs(output)
	state.Poke("output", "root")
	state.Poke(outputInputs, "inputs")

	return state.PackInto(payload)
}

/*
MarshalFeaturesPayload encodes a feature vector as JSON payload bytes.
Prefer MarshalFeatureSchema with explicit input keys for new tests.
*/
func MarshalFeaturesPayload(samples []float64) []byte {
	return MarshalFeatureSchema(nil, samples)
}

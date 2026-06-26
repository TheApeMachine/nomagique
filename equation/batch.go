package equation

import (
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

	for key := range fields {
		outputInputs = append(outputInputs, key)
	}

	for key, value := range fields {
		state.MergeOutput(key, value)
	}

	if _, hasStrength := fields["strength"]; !hasStrength {
		if value, ok := fields["value"]; ok {
			state.MergeOutput("strength", value)
			outputInputs = append(outputInputs, "strength")
		}
	}

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

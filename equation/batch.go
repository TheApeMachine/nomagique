package equation

import (
	"github.com/bytedance/sonic"
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

	if _, err := state.Write(bytes); err != nil {
		state.Release()

		return nil, err
	}

	return state, nil
}

func emitZero(state *datura.Artifact, payload []byte) (int, error) {
	return emitOutput(state, payload, datura.Map[float64]{"value": 0})
}

func emitOutput(state *datura.Artifact, payload []byte, fields datura.Map[float64]) (int, error) {
	defer state.Release()

	for key, value := range fields {
		state.MergeOutput(key, value)
	}

	return state.Read(payload)
}

/*
MarshalFeaturesPayload encodes a feature vector as JSON payload bytes.
*/
func MarshalFeaturesPayload(samples []float64) []byte {
	payload := errnie.Does(func() ([]byte, error) {
		return sonic.Marshal(datura.Map[[]float64]{"features": samples})
	}).Or(func(err error) {
		errnie.Error(errnie.Err(errnie.IO, "equation: marshal features payload", err))
	}).Value()

	return payload
}

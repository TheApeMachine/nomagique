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

package adaptive

import (
	"encoding/binary"
	"math"

	"github.com/theapemachine/datura"
)

func assignScalarPayload(artifact **datura.Artifact, name string, value float64) {
	*artifact = datura.Acquire(name, datura.Artifact_Type_json)
	_ = (*artifact).SetPayload(scalarPayload(value))
}

func scalarPayload(value float64) []byte {
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, math.Float64bits(value))

	return payload
}

func finiteScalar(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func observeScalarPayload(
	artifact **datura.Artifact,
	name string,
	payload []byte,
	step func(float64) float64,
) {
	if len(payload) != 8 {
		return
	}

	sample := math.Float64frombits(binary.BigEndian.Uint64(payload))
	_ = observeScalarSample(artifact, name, sample, step)
}

func observeScalarSample(
	artifact **datura.Artifact,
	name string,
	sample float64,
	step func(float64) float64,
) float64 {
	if !finiteScalar(sample) {
		return valueFromArtifact(*artifact)
	}

	derived := step(sample)

	if !finiteScalar(derived) {
		return valueFromArtifact(*artifact)
	}

	assignScalarPayload(artifact, name, derived)

	return derived
}

func valueFromArtifact(artifact *datura.Artifact) float64 {
	payload, err := artifact.Payload()

	if err != nil || len(payload) != 8 {
		return 0
	}

	return math.Float64frombits(binary.BigEndian.Uint64(payload))
}

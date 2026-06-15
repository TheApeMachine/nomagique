package adaptive

import (
	"encoding/binary"
	"math"

	"github.com/theapemachine/datura"
)

func assignScalarPayload(artifact **datura.Artifact, name string, value float64) {
	*artifact = datura.Acquire(name, datura.Artifact_Type_json)
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, math.Float64bits(value))
	_ = (*artifact).SetPayload(payload)
}

func valueFromArtifact(artifact *datura.Artifact) float64 {
	payload, err := artifact.Payload()

	if err != nil || len(payload) != 8 {
		return 0
	}

	return math.Float64frombits(binary.BigEndian.Uint64(payload))
}

package probability

import (
	"encoding/binary"
	"math"

	"github.com/theapemachine/datura"
)

/*
CUSUM accumulates sequential change evidence from a sample stream.
*/
type CUSUM struct {
	artifact *datura.Artifact
	state    CUSUMState
}

/*
NewCUSUM returns a change-detection stage ready from its first observation.
*/
func NewCUSUM() *CUSUM {
	return &CUSUM{
		artifact: datura.Acquire("cusum", datura.Artifact_Type_json),
	}
}

func (cusum *CUSUM) Write(p []byte) (int, error) {
	return cusum.artifact.Write(p)
}

func (cusum *CUSUM) Read(p []byte) (int, error) {
	rehydrateArtifact(&cusum.artifact, "cusum", datura.Artifact_Type_json)

	payload, err := cusum.artifact.Payload()

	if err == nil && len(payload) == 8 {
		sample := math.Float64frombits(binary.BigEndian.Uint64(payload))
		derived := ObserveCUSUM(&cusum.state, sample)
		out := make([]byte, 8)
		binary.BigEndian.PutUint64(out, math.Float64bits(derived))
		_ = cusum.artifact.SetPayload(out)
	}

	return cusum.artifact.Read(p)
}

func (cusum *CUSUM) Value() float64 {
	payload, _ := cusum.artifact.Payload()
	value, ok := payloadScalar(payload)

	if !ok {
		return 0
	}

	return value
}

func (cusum *CUSUM) Close() error {
	return nil
}

/*
ObserveSamples runs the exact batch kernel over samples into out.
*/
func (cusum *CUSUM) ObserveSamples(samples []float64, out []float64) {
	cusum.state.ObserveSamples(samples, out)
}

/*
Reset clears derived state.
*/
func (cusum *CUSUM) Reset() error {
	cusum.state.Reset()

	return nil
}

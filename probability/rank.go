package probability

import (
	"encoding/binary"
	"math"

	"github.com/theapemachine/datura"
)

/*
Rank tracks P(history <= current sample) over a span-derived window.
*/
type Rank struct {
	artifact *datura.Artifact
	state    RankState
}

/*
NewRank returns an empirical rank probability stage ready from its first observation.
*/
func NewRank() *Rank {
	return &Rank{
		artifact: datura.Acquire("rank", datura.Artifact_Type_json),
	}
}

func (rank *Rank) Write(p []byte) (int, error) {
	return rank.artifact.Write(p)
}

func (rank *Rank) Read(p []byte) (int, error) {
	rehydrateArtifact(&rank.artifact, "rank", datura.Artifact_Type_json)

	payload, err := rank.artifact.Payload()

	if err == nil && len(payload) == 8 {
		sample := math.Float64frombits(binary.BigEndian.Uint64(payload))
		derived := ObserveRank(&rank.state, sample)
		out := make([]byte, 8)
		binary.BigEndian.PutUint64(out, math.Float64bits(derived))
		_ = rank.artifact.SetPayload(out)
	}

	return rank.artifact.Read(p)
}

func (rank *Rank) Value() float64 {
	payload, _ := rank.artifact.Payload()
	value, ok := payloadScalar(payload)

	if !ok {
		return 0
	}

	return value
}

func (rank *Rank) Close() error {
	return nil
}

/*
ObserveSamples runs the exact batch kernel over samples into out.
*/
func (rank *Rank) ObserveSamples(samples []float64, out []float64) {
	rank.state.ObserveSamples(samples, out)
}

/*
Reset clears derived state.
*/
func (rank *Rank) Reset() error {
	rank.state.Reset()

	return nil
}

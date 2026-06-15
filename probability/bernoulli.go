package probability

import (
	"encoding/binary"
	"math"

	"github.com/theapemachine/datura"
)

/*
Bernoulli tracks a Beta posterior mean from Bernoulli outcomes.
*/
type Bernoulli struct {
	artifact *datura.Artifact
	state    BetaState
}

/*
NewBernoulli returns a Beta-Bernoulli stage ready from its first observation.
*/
func NewBernoulli() *Bernoulli {
	return &Bernoulli{
		artifact: datura.Acquire("bernoulli", datura.Artifact_Type_json),
	}
}

func (bernoulli *Bernoulli) Write(p []byte) (int, error) {
	return bernoulli.artifact.Write(p)
}

func (bernoulli *Bernoulli) Read(p []byte) (int, error) {
	rehydrateArtifact(&bernoulli.artifact, "bernoulli", datura.Artifact_Type_json)

	payload, err := bernoulli.artifact.Payload()

	if err == nil && len(payload) >= 8 {
		samples := payloadSamples(payload)

		if len(samples) >= 2 {
			predicted, actual, parseErr := parsePredictedActual(samples[0], samples[1:])

			if parseErr == nil {
				derived := ObserveBetaPair(&bernoulli.state, predicted, actual)
				out := make([]byte, 8)
				binary.BigEndian.PutUint64(out, math.Float64bits(derived))
				_ = bernoulli.artifact.SetPayload(out)
			}
		}

		if len(samples) == 1 {
			outcome, parseErr := parseBernoulliOutcome(samples[0], nil)

			if parseErr == nil {
				derived := ObserveBeta(&bernoulli.state, outcome)
				out := make([]byte, 8)
				binary.BigEndian.PutUint64(out, math.Float64bits(derived))
				_ = bernoulli.artifact.SetPayload(out)
			}

			if parseErr != nil {
				out := make([]byte, 8)
				binary.BigEndian.PutUint64(out, math.Float64bits(0))
				_ = bernoulli.artifact.SetPayload(out)
			}
		}
	}

	return bernoulli.artifact.Read(p)
}

func (bernoulli *Bernoulli) Value() float64 {
	payload, _ := bernoulli.artifact.Payload()
	value, ok := payloadScalar(payload)

	if !ok {
		return 0
	}

	return value
}

func (bernoulli *Bernoulli) Close() error {
	return nil
}

/*
ObserveSamples runs the exact batch kernel over outcomes into out.
*/
func (bernoulli *Bernoulli) ObserveSamples(outcomes []float64, out []float64) {
	bernoulli.state.ObserveSamples(outcomes, out)
}

/*
ObservePairSamples runs the exact batch kernel over pairs into out.
*/
func (bernoulli *Bernoulli) ObservePairSamples(
	predicted []float64, actual []float64, out []float64,
) {
	bernoulli.state.ObservePairSamples(predicted, actual, out)
}

/*
Reset clears derived state.
*/
func (bernoulli *Bernoulli) Reset() error {
	bernoulli.state.Reset()

	return nil
}

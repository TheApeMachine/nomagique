package learning

import (
	"github.com/theapemachine/datura"
)

/*
TrustWeight is a self-adapting rate from prediction error.
*/
type TrustWeight struct {
	artifact *datura.Artifact
	state    WeightState
}

/*
Weight returns a trust weight dynamic ready from its first observation.
*/
func Weight() *TrustWeight {
	return &TrustWeight{
		artifact: datura.Acquire("weight", datura.Artifact_Type_json),
	}
}

func (trustWeight *TrustWeight) Write(p []byte) (int, error) {
	return trustWeight.artifact.Write(p)
}

func (trustWeight *TrustWeight) Read(p []byte) (int, error) {
	values := float64Batch(trustWeight.artifact)

	if len(values) >= 2 {
		predicted, actual, err := parsePredictedActual(values[0], values[1:])

		if err == nil {
			derived := ObserveWeight(&trustWeight.state, predicted, actual)
			putFloat64Payload(&trustWeight.artifact, "weight", derived)
		}
	}

	return trustWeight.artifact.Read(p)
}

func (trustWeight *TrustWeight) Close() error {
	return nil
}

/*
ObserveSamples runs the exact batch kernel over pairs into out.
*/
func (trustWeight *TrustWeight) ObserveSamples(
	predicted []float64, actual []float64, out []float64,
) {
	trustWeight.state.ObserveSamples(predicted, actual, out)
}

/*
Reset clears derived state.
*/
func (trustWeight *TrustWeight) Reset() error {
	trustWeight.state.Reset()

	return nil
}

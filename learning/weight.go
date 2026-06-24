package learning

import (
	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
TrustWeight is a self-adapting rate from prediction error.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type TrustWeight struct {
	artifact *datura.Artifact
}

/*
Weight returns a trust weight stage wired from config attributes on the artifact.
*/
func Weight(artifact *datura.Artifact) *TrustWeight {
	return &TrustWeight{
		artifact: artifact,
	}
}

func (trustWeight *TrustWeight) Read(payload []byte) (int, error) {
	state := datura.Acquire("trust-weight-state", datura.APPJSON)

	if _, err := state.Write(trustWeight.artifact.DecryptPayload()); err != nil {
		state.Release()

		return 0, err
	}

	state.Inspect("learning", "trust-weight", "Read()", "p")
	defer state.Release()

	predicted, actual, err := trustWeight.resolvePair(state)

	if err != nil {
		return 0, err
	}

	weightState := weightStateFromArtifact(trustWeight.artifact)
	derived := ObserveWeight(&weightState, predicted, actual)
	pokeWeightState(trustWeight.artifact, &weightState, derived)
	state.MergeOutput("value", derived)
	state.MergeOutput("predicted", predicted)
	state.MergeOutput("actual", actual)
	state.Poke("output", "root")
	state.Poke([]string{"value", "predicted", "actual"}, "inputs")
	return state.Read(payload)
}

func (trustWeight *TrustWeight) resolvePair(state *datura.Artifact) (float64, float64, error) {
	parsedPredicted, parsedActual, err := wirePair(trustWeight.artifact, state, "trust-weight")

	if err != nil {
		return 0, 0, err
	}

	if parsedActual == 0 {
		return 0, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"trust-weight: actual must be non-zero",
			nil,
		))
	}

	return parsedPredicted, parsedActual, nil
}

func (trustWeight *TrustWeight) Write(payload []byte) (int, error) {
	trustWeight.artifact.WithPayload(payload)
	return len(payload), nil
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
	weightState := weightStateFromArtifact(trustWeight.artifact)
	observeWeightSamples(&weightState, predicted, actual, out)

	if len(out) > 0 {
		pokeWeightState(trustWeight.artifact, &weightState, out[len(out)-1])
	}
}

/*
Reset clears derived state.
*/
func (trustWeight *TrustWeight) Reset() error {
	trustWeight.artifact.Poke(0.0, "output", "trust")
	trustWeight.artifact.Poke(0.0, "output", "prev")
	trustWeight.artifact.Poke(0.0, "output", "min")
	trustWeight.artifact.Poke(0.0, "output", "max")
	trustWeight.artifact.Poke(0.0, "output", "rate")
	trustWeight.artifact.Poke(0.0, "output", "ready")
	trustWeight.artifact.Poke(0.0, "output", "value")

	return nil
}

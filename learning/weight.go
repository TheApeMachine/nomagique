package learning

import (
	"github.com/theapemachine/datura"
)

/*
TrustWeight is a self-adapting rate from prediction error.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type TrustWeight struct {
	artifact *datura.Artifact
	state    WeightState
}

/*
Weight returns a trust weight stage wired from config attributes on the artifact.
*/
func Weight(artifact *datura.Artifact) *TrustWeight {
	artifact.Inspect("learning", "trust-weight", "Weight()")

	return &TrustWeight{
		artifact: artifact,
	}
}

func (trustWeight *TrustWeight) Write(payload []byte) (int, error) {
	trustWeight.artifact.WithPayload(payload)
	return len(payload), nil
}

func (trustWeight *TrustWeight) Read(payload []byte) (int, error) {
	state := datura.Acquire("trust-weight-state", datura.APPJSON)
	state.Inspect("learning", "trust-weight", "Read()", "p")

	if _, err := state.Write(trustWeight.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	predicted := datura.Peek[float64](state, "sample")
	actual := datura.Peek[float64](state, "paired")

	if predicted == 0 && actual == 0 {
		features := datura.Peek[[]float64](state, "features")

		if len(features) >= 2 {
			predicted = features[0]
			actual = features[1]
		}
	}

	if predicted == 0 && actual == 0 {
		value := datura.Peek[float64](trustWeight.artifact, "output", "value")

		if trustWeight.state.Ready {
			state.MergeOutput("value", value)
			state.Merge("root", "output")
			state.Merge("inputs", []string{"value"})
		}

		return state.Read(payload)
	}

	if actual == 0 {
		return state.Read(payload)
	}

	parsedPredicted, parsedActual, err := parsePredictedActual(predicted, []float64{actual})

	if err != nil {
		return state.Read(payload)
	}

	derived := ObserveWeight(&trustWeight.state, parsedPredicted, parsedActual)
	trustWeight.artifact.Poke(derived, "output", "value")
	state.MergeOutput("value", derived)
	state.Merge("sample", parsedPredicted)
	state.Merge("paired", parsedActual)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value", "sample", "paired"})
	return state.Read(payload)
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
	trustWeight.artifact.WithAttributes(datura.Map[any]{})

	return nil
}

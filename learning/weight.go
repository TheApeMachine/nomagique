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
		artifact: datura.Acquire("weight", datura.APPJSON).RetainStageAttributes(),
	}
}

func (trustWeight *TrustWeight) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](trustWeight.artifact, "output") == nil

	trustWeight.artifact.Clear("sample")
	trustWeight.artifact.Clear("paired")

	n, err := trustWeight.artifact.Write(p)

	if bootstrap {
		trustWeight.artifact.Clear("output")
	}

	return n, err
}

func (trustWeight *TrustWeight) Read(p []byte) (int, error) {
	predicted := datura.Peek[float64](trustWeight.artifact, "sample")
	actual := datura.Peek[float64](trustWeight.artifact, "paired")

	if predicted == 0 && actual == 0 {
		return trustWeight.artifact.Read(p)
	}

	if actual == 0 {
		return trustWeight.artifact.Read(p)
	}

	parsedPredicted, parsedActual, err := parsePredictedActual(predicted, []float64{actual})

	if err != nil {
		return trustWeight.artifact.Read(p)
	}

	derived := ObserveWeight(&trustWeight.state, parsedPredicted, parsedActual)
	trustWeight.artifact.Poke(datura.Map[float64]{"value": derived}, "output")

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
	trustWeight.artifact.Clear("output")

	return nil
}

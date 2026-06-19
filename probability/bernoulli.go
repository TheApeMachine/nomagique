package probability

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
Bernoulli tracks a Beta posterior mean from Bernoulli outcomes.
*/
type Bernoulli struct {
	artifact *datura.Artifact
}

/*
NewBernoulli returns a Beta-Bernoulli stage ready from its first observation.
*/
func NewBernoulli() *Bernoulli {
	return &Bernoulli{
		artifact: datura.Acquire("bernoulli", datura.APPJSON),
	}
}

func (bernoulli *Bernoulli) Write(p []byte) (int, error) {
	return bernoulli.artifact.Write(p)
}

func (bernoulli *Bernoulli) Read(p []byte) (int, error) {
	pairedPresent := attributeKeyPresent(bernoulli.artifact, "paired")
	samplePresent := attributeKeyPresent(bernoulli.artifact, "sample")

	if !samplePresent && !pairedPresent {
		return bernoulli.artifact.Read(p)
	}

	sample := datura.Peek[float64](bernoulli.artifact, "sample")

	output := datura.Peek[datura.Map[float64]](bernoulli.artifact, "output")
	state := BetaState{}

	if output != nil {
		state.Alpha = output["alpha"]
		state.Beta = output["beta"]
		state.Prev = output["prev"]
		state.Min = output["min"]
		state.Max = output["max"]
		state.Rate = output["rate"]
		state.Ready = output["ready"] != 0
	}

	value := 0.0

	if pairedPresent {
		paired := datura.Peek[float64](bernoulli.artifact, "paired")
		predicted, actual, parseErr := parsePredictedActual(sample, []float64{paired})

		if parseErr == nil {
			value = ObserveBetaPair(&state, predicted, actual)
		}
	}

	if !pairedPresent {
		if math.IsNaN(sample) || math.IsInf(sample, 0) {
			return bernoulli.artifact.Read(p)
		}

		outcome, parseErr := parseBernoulliOutcome(sample, nil)

		if parseErr == nil {
			value = ObserveBeta(&state, outcome)
		}
	}

	ready := 0.0

	if state.Ready {
		ready = 1
	}

	bernoulli.artifact.Poke(datura.Map[float64]{
		"alpha": state.Alpha,
		"beta":  state.Beta,
		"prev":  state.Prev,
		"min":   state.Min,
		"max":   state.Max,
		"rate":  state.Rate,
		"ready": ready,
		"value": value,
	}, "output")

	return bernoulli.artifact.Read(p)
}

func (bernoulli *Bernoulli) Close() error {
	return nil
}

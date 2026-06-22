package probability

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Bernoulli tracks a Beta posterior mean from Bernoulli outcomes.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Bernoulli struct {
	artifact *datura.Artifact
}

/*
NewBernoulli returns a Beta-Bernoulli stage wired from config attributes on the artifact.
*/
func NewBernoulli(artifact *datura.Artifact) *Bernoulli {
	return &Bernoulli{
		artifact: artifact,
	}
}

func (bernoulli *Bernoulli) Write(payload []byte) (int, error) {
	bernoulli.artifact.WithPayload(payload)
	return len(payload), nil
}

func (bernoulli *Bernoulli) Read(payload []byte) (int, error) {
	state := datura.Acquire("bernoulli-state", datura.APPJSON)

	if _, err := state.Write(bernoulli.artifact.DecryptPayload()); err != nil {
		state.Release()

		return 0, err
	}

	defer state.Release()

	if datura.Peek[float64](state, "reset") != 0 {
		bernoulli.artifact.Poke(0.0, "output", "alpha")
		bernoulli.artifact.Poke(0.0, "output", "beta")
		bernoulli.artifact.Poke(0.0, "output", "prev")
		bernoulli.artifact.Poke(0.0, "output", "min")
		bernoulli.artifact.Poke(0.0, "output", "max")
		bernoulli.artifact.Poke(0.0, "output", "rate")
		bernoulli.artifact.Poke(0.0, "output", "ready")
		bernoulli.artifact.Poke(0.0, "output", "value")
		state.MergeOutput("ready", 0)
		state.MergeOutput("value", 0)
		state.Merge("root", "output")
		state.Merge("inputs", []string{"value"})
		return state.Read(payload)
	}

	samplePresent := attributeKeyPresent(state, "sample")
	pairedPresent := attributeKeyPresent(state, "paired")

	if !samplePresent && !pairedPresent {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"bernoulli: sample or paired required",
			nil,
		))
	}

	betaState := BetaState{
		Alpha: datura.Peek[float64](bernoulli.artifact, "output", "alpha"),
		Beta:  datura.Peek[float64](bernoulli.artifact, "output", "beta"),
		Prev:  datura.Peek[float64](bernoulli.artifact, "output", "prev"),
		Min:   datura.Peek[float64](bernoulli.artifact, "output", "min"),
		Max:   datura.Peek[float64](bernoulli.artifact, "output", "max"),
		Rate:  datura.Peek[float64](bernoulli.artifact, "output", "rate"),
		Ready: datura.Peek[float64](bernoulli.artifact, "output", "ready") != 0,
	}

	sample := datura.Peek[float64](state, "sample")
	value := 0.0

	if pairedPresent {
		paired := datura.Peek[float64](state, "paired")
		predicted, actual, parseErr := parsePredictedActual(sample, []float64{paired})

		if parseErr != nil {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"bernoulli: unable to parse predicted and actual pair",
				parseErr,
			))
		}

		value = ObserveBetaPair(&betaState, predicted, actual)
	}

	if !pairedPresent {
		if math.IsNaN(sample) || math.IsInf(sample, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"bernoulli: sample is non-finite",
				nil,
			))
		}

		outcome, parseErr := parseBernoulliOutcome(sample, nil)

		if parseErr != nil {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"bernoulli: invalid outcome",
				parseErr,
			))
		}

		value = ObserveBeta(&betaState, outcome)
	}

	ready := 0.0

	if betaState.Ready {
		ready = 1
	}

	bernoulli.artifact.Poke(betaState.Alpha, "output", "alpha")
	bernoulli.artifact.Poke(betaState.Beta, "output", "beta")
	bernoulli.artifact.Poke(betaState.Prev, "output", "prev")
	bernoulli.artifact.Poke(betaState.Min, "output", "min")
	bernoulli.artifact.Poke(betaState.Max, "output", "max")
	bernoulli.artifact.Poke(betaState.Rate, "output", "rate")
	bernoulli.artifact.Poke(ready, "output", "ready")
	bernoulli.artifact.Poke(value, "output", "value")
	state.MergeOutput("value", value)
	state.MergeOutput("ready", ready)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(payload)
}

func (bernoulli *Bernoulli) Close() error {
	return nil
}

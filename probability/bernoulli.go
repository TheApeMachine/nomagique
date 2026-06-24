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
	beta     BetaState
}

/*
NewBernoulli returns a Beta-Bernoulli stage wired from config attributes on the artifact.
*/
func NewBernoulli(artifact *datura.Artifact) *Bernoulli {
	return &Bernoulli{
		artifact: artifact,
	}
}

func (bernoulli *Bernoulli) Read(payload []byte) (int, error) {
	state := datura.Acquire("bernoulli-state", datura.APPJSON)

	if _, err := state.Write(bernoulli.artifact.DecryptPayload()); err != nil {
		state.Release()

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"bernoulli: state write failed",
			err,
		))
	}

	defer state.Release()

	if datura.Peek[float64](state, "reset") != 0 {
		bernoulli.beta.Reset()
		state.MergeOutput("ready", 0)
		state.MergeOutput("value", 0)
		state.Poke("output", "root")
		state.Poke([]string{"value"}, "inputs")

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"bernoulli: reset",
			nil,
		))
	}

	sampleKey := datura.Peek[string](bernoulli.artifact, "sampleKey")

	if sampleKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"bernoulli: sampleKey required",
			nil,
		))
	}

	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"bernoulli: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"bernoulli: inputs required",
			nil,
		))
	}

	var sample float64
	sampleFound := false

	for index, input := range inputs {
		if input != sampleKey {
			continue
		}

		if rootKey == "features" {
			features := datura.Peek[[]float64](state, rootKey)

			if index >= len(features) {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"bernoulli: feature index out of range",
					nil,
				))
			}

			sample = features[index]
		}

		if rootKey != "features" {
			sample = datura.Peek[float64](state, rootKey, input)
		}

		sampleFound = true
	}

	if !sampleFound {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"bernoulli: sampleKey not in inputs",
			nil,
		))
	}

	pairedKey := datura.Peek[string](bernoulli.artifact, "pairedKey")
	value := 0.0

	if pairedKey != "" {
		var paired float64
		pairedFound := false

		for index, input := range inputs {
			if input != pairedKey {
				continue
			}

			if rootKey == "features" {
				features := datura.Peek[[]float64](state, rootKey)

				if index >= len(features) {
					return 0, errnie.Error(errnie.Err(
						errnie.Validation,
						"bernoulli: feature index out of range",
						nil,
					))
				}

				paired = features[index]
			}

			if rootKey != "features" {
				paired = datura.Peek[float64](state, rootKey, input)
			}

			pairedFound = true
		}

		if !pairedFound {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"bernoulli: pairedKey not in inputs",
				nil,
			))
		}

		predicted, actual, err := parsePredictedActual(sample, []float64{paired})

		if err != nil {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"bernoulli: unable to parse predicted and actual pair",
				err,
			))
		}

		value = bernoulli.beta.ObservePair(predicted, actual)
	}

	if pairedKey == "" {
		if math.IsNaN(sample) || math.IsInf(sample, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"bernoulli: sample is non-finite",
				nil,
			))
		}

		outcome, err := parseBernoulliOutcome(sample, nil)

		if err != nil {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"bernoulli: invalid outcome",
				err,
			))
		}

		value = bernoulli.beta.Observe(outcome)
	}

	ready := 0.0

	if bernoulli.beta.Ready {
		ready = 1
	}

	state.MergeOutput("value", value)
	state.MergeOutput("ready", ready)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")

	return state.Read(payload)
}

func (bernoulli *Bernoulli) Write(payload []byte) (int, error) {
	bernoulli.artifact.WithPayload(payload)
	return len(payload), nil
}

func (bernoulli *Bernoulli) Close() error {
	return nil
}

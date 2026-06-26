package probability

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat/distuv"
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

func (bernoulli *Bernoulli) Read(payload []byte) (int, error) {
	state := datura.Acquire("bernoulli-state", datura.APPJSON)

	if _, err := state.Unpack(bernoulli.artifact.DecryptPayload()); err != nil {
		state.Release()

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"bernoulli: state write failed",
			err,
		))
	}

	defer state.Release()

	if datura.Peek[float64](state, "reset") != 0 {
		bernoulli.artifact.Poke(0.0, "output", "alpha")
		bernoulli.artifact.Poke(0.0, "output", "beta")
		bernoulli.artifact.Poke(0.0, "output", "prev")
		bernoulli.artifact.Poke(0.0, "output", "min")
		bernoulli.artifact.Poke(0.0, "output", "max")
		bernoulli.artifact.Poke(0.0, "output", "rate")
		bernoulli.artifact.Poke(0.0, "output", "count")
		bernoulli.artifact.Poke(0.0, "output", "value")
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

	alpha := datura.Peek[float64](bernoulli.artifact, "output", "alpha")
	betaParam := datura.Peek[float64](bernoulli.artifact, "output", "beta")
	prev := datura.Peek[float64](bernoulli.artifact, "output", "prev")
	minimum := datura.Peek[float64](bernoulli.artifact, "output", "min")
	maximum := datura.Peek[float64](bernoulli.artifact, "output", "max")
	rate := datura.Peek[float64](bernoulli.artifact, "output", "rate")
	count := int(datura.Peek[float64](bernoulli.artifact, "output", "count"))
	pairedKey := datura.Peek[string](bernoulli.artifact, "pairedKey")

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

		success := 0.0

		if actual >= predicted {
			success = 1
		}

		tracking := actual - predicted

		if count == 0 {
			alpha = 1 + success
			betaParam = 1 + (1 - success)
			prev = predicted
			minimum = tracking
			maximum = tracking
			count = 1
		}

		if count > 1 {
			minimum = math.Min(minimum, tracking)
			maximum = math.Max(maximum, tracking)
			count++
		}

		if count == 1 && tracking != minimum {
			minimum = math.Min(minimum, tracking)
			maximum = math.Max(maximum, tracking)
			count = 2
		}

		span := maximum - minimum

		if count > 1 {
			if span == 0 {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"bernoulli: sample span is zero",
					nil,
				))
			}

			rate = math.Abs(tracking) / span
			prev = predicted
			alpha += rate * success
			betaParam += rate * (1 - success)
		}
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

		if count == 0 {
			alpha = 1 + outcome
			betaParam = 1 + (1 - outcome)
			prev = outcome
			minimum = outcome
			maximum = outcome
			count = 1
		}

		if count > 1 {
			minimum = math.Min(minimum, outcome)
			maximum = math.Max(maximum, outcome)
			count++
		}

		if count == 1 && outcome != minimum {
			minimum = math.Min(minimum, outcome)
			maximum = math.Max(maximum, outcome)
			count = 2
		}

		span := maximum - minimum

		if count > 1 {
			if span == 0 {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"bernoulli: sample span is zero",
					nil,
				))
			}

			movement := outcome - prev
			rate = math.Abs(movement) / span
			prev = outcome
			alpha += rate * outcome
			betaParam += rate * (1 - outcome)
		}
	}

	if alpha+betaParam == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"bernoulli: posterior is undefined",
			nil,
		))
	}

	distribution := distuv.Beta{
		Alpha: alpha,
		Beta:  betaParam,
	}
	value := distribution.Mean()

	bernoulli.artifact.Poke(alpha, "output", "alpha")
	bernoulli.artifact.Poke(betaParam, "output", "beta")
	bernoulli.artifact.Poke(prev, "output", "prev")
	bernoulli.artifact.Poke(minimum, "output", "min")
	bernoulli.artifact.Poke(maximum, "output", "max")
	bernoulli.artifact.Poke(rate, "output", "rate")
	bernoulli.artifact.Poke(float64(count), "output", "count")
	bernoulli.artifact.Poke(value, "output", "value")
	state.MergeOutput("value", value)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")

	return state.PackInto(payload)
}

func (bernoulli *Bernoulli) Write(payload []byte) (int, error) {
	bernoulli.artifact.WithPayload(payload)
	return len(payload), nil
}

func (bernoulli *Bernoulli) Close() error {
	return nil
}

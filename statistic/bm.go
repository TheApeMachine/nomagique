package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
BivariateMoment computes the bivariate moment between paired sample streams.
*/
type BivariateMoment struct {
	artifact *datura.Artifact
}

/*
NewBivariateMoment returns a bivariate-moment stage wired from config attributes on the artifact.
*/
func NewBivariateMoment(artifact *datura.Artifact) *BivariateMoment {
	return &BivariateMoment{
		artifact: artifact,
	}
}

func (bivariateMoment *BivariateMoment) Read(payload []byte) (int, error) {
	state := datura.Acquire("bivariate-moment-state", datura.APPJSON)

	if _, err := state.Write(bivariateMoment.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"bivariate-moment: state write failed",
			err,
		))
	}

	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"bivariate-moment: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"bivariate-moment: inputs required",
			nil,
		))
	}

	sampleKey := datura.Peek[string](bivariateMoment.artifact, "sampleKey")

	if sampleKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"bivariate-moment: sampleKey required",
			nil,
		))
	}

	pairedKey := datura.Peek[string](bivariateMoment.artifact, "pairedKey")

	if pairedKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"bivariate-moment: pairedKey required",
			nil,
		))
	}

	outputKey := datura.Peek[string](bivariateMoment.artifact, "outputKey")

	if outputKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"bivariate-moment: outputKey required",
			nil,
		))
	}

	var sample float64
	var paired float64
	sampleFound := false
	pairedFound := false

	for index, input := range inputs {
		if rootKey == "features" {
			features := datura.Peek[[]float64](state, rootKey)

			if index >= len(features) {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"bivariate-moment: feature index out of range",
					nil,
				))
			}

			if input == sampleKey {
				sample = features[index]
				sampleFound = true
			}

			if input == pairedKey {
				paired = features[index]
				pairedFound = true
			}
		}

		if rootKey != "features" {
			if input == sampleKey {
				sample = datura.Peek[float64](state, rootKey, input)
				sampleFound = true
			}

			if input == pairedKey {
				paired = datura.Peek[float64](state, rootKey, input)
				pairedFound = true
			}
		}
	}

	if !sampleFound || !pairedFound {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"bivariate-moment: sampleKey or pairedKey not in inputs",
			nil,
		))
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) || math.IsNaN(paired) || math.IsInf(paired, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"bivariate-moment: sample and paired must be finite",
			nil,
		))
	}

	samples := datura.Peek[[]float64](bivariateMoment.artifact, "samples")
	pairedSamples := datura.Peek[[]float64](bivariateMoment.artifact, "paired")
	samples = append(samples, sample)
	pairedSamples = append(pairedSamples, paired)
	bivariateMoment.artifact.Poke(samples, "samples")
	bivariateMoment.artifact.Poke(pairedSamples, "paired")

	momentR := datura.Peek[float64](bivariateMoment.artifact, "config", "r")
	momentS := datura.Peek[float64](bivariateMoment.artifact, "config", "s")

	if len(samples) < 2 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"bivariate-moment: insufficient paired samples",
			nil,
		))
	}

	value := stat.BivariateMoment(momentR, momentS, samples, pairedSamples, nil)

	state.MergeOutput(outputKey, value)
	state.Poke("output", "root")
	state.Poke([]string{outputKey}, "inputs")

	return state.Read(payload)
}

func (bivariateMoment *BivariateMoment) Write(payload []byte) (int, error) {
	bivariateMoment.artifact.WithPayload(payload)
	return len(payload), nil
}

func (bivariateMoment *BivariateMoment) Close() error {
	return nil
}

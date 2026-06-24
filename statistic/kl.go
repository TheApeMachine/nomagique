package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
KLDivergence computes KL divergence between paired sample streams.
*/
type KLDivergence struct {
	artifact *datura.Artifact
}

/*
NewKLDivergence returns a KL-divergence stage wired from config attributes on the artifact.
*/
func NewKLDivergence(artifact *datura.Artifact) *KLDivergence {
	return &KLDivergence{
		artifact: artifact,
	}
}

func (klDivergence *KLDivergence) Read(payload []byte) (int, error) {
	state := datura.Acquire("kl-divergence-state", datura.APPJSON)

	if _, err := state.Write(klDivergence.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"kl-divergence: state write failed",
			err,
		))
	}

	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"kl-divergence: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"kl-divergence: inputs required",
			nil,
		))
	}

	sampleKey := datura.Peek[string](klDivergence.artifact, "sampleKey")

	if sampleKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"kl-divergence: sampleKey required",
			nil,
		))
	}

	pairedKey := datura.Peek[string](klDivergence.artifact, "pairedKey")

	if pairedKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"kl-divergence: pairedKey required",
			nil,
		))
	}

	outputKey := datura.Peek[string](klDivergence.artifact, "outputKey")

	if outputKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"kl-divergence: outputKey required",
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
					"kl-divergence: feature index out of range",
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
			"kl-divergence: sampleKey or pairedKey not in inputs",
			nil,
		))
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) || math.IsNaN(paired) || math.IsInf(paired, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"kl-divergence: sample and paired must be finite",
			nil,
		))
	}

	if sample < 0 || paired < 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"kl-divergence: sample and paired must be non-negative",
			nil,
		))
	}

	samples := datura.Peek[[]float64](klDivergence.artifact, "samples")
	pairedSamples := datura.Peek[[]float64](klDivergence.artifact, "paired")
	samples = append(samples, sample)
	pairedSamples = append(pairedSamples, paired)
	klDivergence.artifact.Poke(samples, "samples")
	klDivergence.artifact.Poke(pairedSamples, "paired")

	if len(samples) < 2 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"kl-divergence: insufficient paired samples",
			nil,
		))
	}

	totalSample := 0.0
	totalPaired := 0.0

	for index := range samples {
		totalSample += samples[index]
		totalPaired += pairedSamples[index]
	}

	if totalSample <= 0 || totalPaired <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"kl-divergence: totals must be positive",
			nil,
		))
	}

	probabilityFloor := min(totalSample, totalPaired) / float64(len(samples))

	if probabilityFloor <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"kl-divergence: probability floor is non-positive",
			nil,
		))
	}

	divergence := 0.0

	for index := range samples {
		probabilitySample := max(samples[index], probabilityFloor) / totalSample
		probabilityPaired := max(pairedSamples[index], probabilityFloor) / totalPaired
		divergence += probabilitySample * math.Log(probabilitySample/probabilityPaired)
	}

	state.MergeOutput(outputKey, divergence)
	state.Poke("output", "root")
	state.Poke([]string{outputKey}, "inputs")

	return state.Read(payload)
}

func (klDivergence *KLDivergence) Write(payload []byte) (int, error) {
	klDivergence.artifact.WithPayload(payload)
	return len(payload), nil
}

func (klDivergence *KLDivergence) Close() error {
	return nil
}

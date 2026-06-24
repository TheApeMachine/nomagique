package statistic

import (
	"math"
	"sort"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
Quantile computes a sample quantile over retained history.
*/
type Quantile struct {
	artifact *datura.Artifact
}

/*
NewQuantile returns a quantile stage wired from config attributes on the artifact.
*/
func NewQuantile(artifact *datura.Artifact) *Quantile {
	return &Quantile{
		artifact: artifact,
	}
}

func (quantile *Quantile) Read(payload []byte) (int, error) {
	state := datura.Acquire("quantile-state", datura.APPJSON)

	if _, err := state.Write(quantile.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"quantile: state write failed",
			err,
		))
	}

	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"quantile: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"quantile: inputs required",
			nil,
		))
	}

	configInput := datura.Peek[string](quantile.artifact, "input")

	if configInput == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"quantile: input required",
			nil,
		))
	}

	outputKey := datura.Peek[string](quantile.artifact, "outputKey")

	if outputKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"quantile: outputKey required",
			nil,
		))
	}

	var sample float64
	found := false

	for index, input := range inputs {
		if input != configInput {
			continue
		}

		if rootKey == "features" {
			features := datura.Peek[[]float64](state, rootKey)

			if index >= len(features) {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"quantile: feature index out of range",
					nil,
				))
			}

			sample = features[index]
		}

		if rootKey != "features" {
			sample = datura.Peek[float64](state, rootKey, input)
		}

		found = true
	}

	if !found {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"quantile: input not in inputs",
			nil,
		))
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"quantile: sample is non-finite",
			nil,
		))
	}

	history := datura.Peek[[]float64](quantile.artifact, "history")
	history = append(history, sample)
	quantile.artifact.Poke(history, "history")

	sorted := append([]float64(nil), history...)
	sort.Float64s(sorted)

	percentile := datura.Peek[float64](quantile.artifact, "config", "percentile")
	kind := stat.CumulantKind(int(datura.Peek[float64](quantile.artifact, "config", "kind")))
	value := sorted[0]

	if percentile <= 0 {
		value = sorted[0]
	}

	if percentile > 0 && percentile < 1 {
		for _, entry := range sorted {
			if math.IsNaN(entry) || math.IsInf(entry, 0) {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"quantile: history contains non-finite values",
					nil,
				))
			}
		}

		value = stat.Quantile(percentile, kind, sorted, nil)
	}

	if percentile >= 1 {
		value = sorted[len(sorted)-1]
	}

	state.MergeOutput(outputKey, value)
	state.Poke("output", "root")
	state.Poke([]string{outputKey}, "inputs")

	return state.Read(payload)
}

func (quantile *Quantile) Write(payload []byte) (int, error) {
	quantile.artifact.WithPayload(payload)
	return len(payload), nil
}

func (quantile *Quantile) Close() error {
	return nil
}

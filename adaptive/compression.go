package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Compression scores how far below the running baseline the current sample sits.
*/
type Compression struct {
	artifact *datura.Artifact
}

/*
NewCompression returns a compression stage wired from config attributes on the artifact.
*/
func NewCompression(artifact *datura.Artifact) *Compression {
	return &Compression{
		artifact: artifact,
	}
}

func (compression *Compression) Read(payload []byte) (int, error) {
	state := datura.Acquire("compression-state", datura.APPJSON)

	if _, err := state.Write(compression.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"compression: state write failed",
			err,
		))
	}

	inputKey := datura.Peek[string](compression.artifact, "compression", "input")
	outputKey := datura.Peek[string](compression.artifact, "compression", "outputKey")
	seriesKey := datura.Peek[string](compression.artifact, "compression", "seriesKey")

	if inputKey == "" || outputKey == "" || seriesKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"compression: input, outputKey, and seriesKey required",
			nil,
		))
	}

	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"compression: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"compression: inputs required",
			nil,
		))
	}

	var sample float64
	found := false

	for index, input := range inputs {
		if input != inputKey {
			continue
		}

		if rootKey == "features" {
			features := datura.Peek[[]float64](state, rootKey)

			if index >= len(features) {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"compression: feature index out of range",
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
			"compression: input not in inputs",
			nil,
		))
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"compression: sample is non-finite",
			nil,
		))
	}

	baseline := datura.Peek[float64](compression.artifact, "output", "baseline", seriesKey)
	value := 0.0

	if baseline <= 0 {
		compression.artifact.Poke(sample, "output", "baseline", seriesKey)
		state.MergeOutput(outputKey, value)
		state.Poke("output", "root")
		state.Poke([]string{outputKey}, "inputs")

		return state.Read(payload)
	}

	if sample > baseline {
		compression.artifact.Poke(sample, "output", "baseline", seriesKey)
		state.MergeOutput(outputKey, value)
		state.Poke("output", "root")
		state.Poke([]string{outputKey}, "inputs")

		return state.Read(payload)
	}

	value = (baseline - sample) / baseline

	state.MergeOutput(outputKey, value)
	state.Poke("output", "root")
	state.Poke([]string{outputKey}, "inputs")

	return state.Read(payload)
}

func (compression *Compression) Write(payload []byte) (int, error) {
	compression.artifact.WithPayload(payload)
	return len(payload), nil
}

func (compression *Compression) Close() error {
	return nil
}

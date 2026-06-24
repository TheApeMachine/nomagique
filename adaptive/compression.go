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
	artifact  *datura.Artifact
	baselines map[string]float64
}

/*
NewCompression returns a compression stage wired from config attributes on the artifact.
*/
func NewCompression(artifact *datura.Artifact) *Compression {
	return &Compression{
		artifact:  artifact,
		baselines: map[string]float64{},
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

	state.Inspect("adaptive", "compression", "Read()", "p")

	inputKey := datura.Peek[string](compression.artifact, "compression", "input")
	outputKey := datura.Peek[string](compression.artifact, "compression", "outputKey")

	if inputKey == "" || outputKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"compression: input and outputKey required",
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

	seriesKey := "compression"
	scope, _ := state.Scope()

	if scope != "" {
		seriesKey = "compression/" + scope
	}

	baseline := compression.baselines[seriesKey]

	if baseline <= 0 {
		compression.baselines[seriesKey] = sample
		state.MergeOutput(outputKey, 0.0)
		state.Poke("output", "root")
		state.Poke([]string{outputKey}, "inputs")

		return state.Read(payload)
	}

	if sample > baseline {
		compression.baselines[seriesKey] = sample
		state.MergeOutput(outputKey, 0.0)
		state.Poke("output", "root")
		state.Poke([]string{outputKey}, "inputs")

		return state.Read(payload)
	}

	value := (baseline - sample) / baseline

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

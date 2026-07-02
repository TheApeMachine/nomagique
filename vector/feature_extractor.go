package vector

import (
	"io"
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/adaptive"
)

/*
FeatureExtractor is an atomic compute primitive (io.ReadWriteCloser).

The constructor artifact is config (attributes); its payload buses inbound wire
from Write to Read. Read unpacks the frame, extracts features, and emits output.
*/
type FeatureExtractor struct {
	artifact   *datura.Artifact
	transforms map[string]func(*datura.Artifact) io.ReadWriteCloser
	writes     []byte
}

/*
NewFeatureExtractor builds an extractor wired from schema attributes.
*/
func NewFeatureExtractor(artifact *datura.Artifact) *FeatureExtractor {
	return &FeatureExtractor{
		artifact: artifact,
		transforms: map[string]func(*datura.Artifact) io.ReadWriteCloser{
			"ema": func(config *datura.Artifact) io.ReadWriteCloser {
				return adaptive.NewEMA(config)
			},
		},
	}
}

func (extractor *FeatureExtractor) Read(payload []byte) (int, error) {
	state := datura.Acquire("feature-extractor", datura.APPJSON)

	if _, err := state.Unpack(extractor.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"feature-extractor: state write failed",
			err,
		))
	}

	extractor.writes = nil

	role := datura.Peek[string](state, "channel")

	rootKey := datura.Peek[string](extractor.artifact, "root")
	inputs := datura.Peek[[]string](extractor.artifact, "inputs")
	elementIndex := int(datura.Peek[float64](extractor.artifact, "elementIndex"))

	if role != "" {
		if scopedRoot := datura.Peek[string](extractor.artifact, role, "root"); scopedRoot != "" {
			rootKey = scopedRoot
		}

		if scopedInputs := datura.Peek[[]string](extractor.artifact, role, "inputs"); len(scopedInputs) > 0 {
			inputs = scopedInputs
		}

		if scopedIndex := int(datura.Peek[float64](extractor.artifact, role, "elementIndex")); scopedIndex != 0 {
			elementIndex = scopedIndex
		}
	}

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"feature-extractor: root required",
			nil,
		))
	}

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"feature-extractor: inputs required",
			nil,
		))
	}

	features := make([]float64, len(inputs))

	for index, input := range inputs {
		sample := datura.Peek[float64](state, rootKey, elementIndex, input)

		if rootKey == "." {
			sample = datura.Peek[float64](state, input)
		}

		if math.IsNaN(sample) || math.IsInf(sample, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"feature-extractor: sample is non-finite",
				nil,
			))
		}

		transform := datura.Peek[string](extractor.artifact, "transforms", input)

		if role != "" {
			if scopedTransform := datura.Peek[string](extractor.artifact, role, "transforms", input); scopedTransform != "" {
				transform = scopedTransform
			}
		}

		if transform == "" {
			features[index] = sample

			continue
		}

		transformer, ok := extractor.transforms[transform]

		if !ok {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"feature-extractor: transform not registered",
				nil,
			))
		}

		scratch := datura.Acquire("feature-extractor-scratch", datura.APPJSON)
		scratch.WithPayload(datura.Map[any]{"sample": sample}.Marshal())

		config := datura.Acquire("feature-extractor-ema", datura.APPJSON)

		if err := nomagique.RoundTripArtifact(scratch, transformer(config)); err != nil {
			scratch.Release()
			config.Release()

			return 0, err
		}

		config.Release()

		scratchRoot := datura.Peek[string](scratch, "root")

		if scratchRoot == "" {
			scratch.Release()

			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"feature-extractor: transform root required",
				nil,
			))
		}

		scratchInputs := datura.Peek[[]string](scratch, "inputs")

		if len(scratchInputs) == 0 {
			scratch.Release()

			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"feature-extractor: transform inputs required",
				nil,
			))
		}

		transformOutput := ""
		transformFound := false

		for _, scratchInput := range scratchInputs {
			if scratchInput != "value" {
				continue
			}

			transformOutput = scratchInput
			transformFound = true
		}

		if !transformFound {
			scratch.Release()

			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"feature-extractor: transform value output required",
				nil,
			))
		}

		sample = datura.Peek[float64](scratch, scratchRoot, transformOutput)
		scratch.Release()

		features[index] = sample
	}

	state.Merge("features", features)
	state.Poke("features", "root")
	state.Poke(inputs, "inputs")
	state.Poke(inputs, "featureInputs")
	state.Poke(rootKey, "sourceRoot")
	state.Poke(inputs, "sourceInputs")

	return state.PackInto(payload)
}

func (extractor *FeatureExtractor) Write(p []byte) (int, error) {
	extractor.writes = append(extractor.writes, p...)
	extractor.artifact.WithPayload(extractor.writes)

	return len(p), nil
}

func (extractor *FeatureExtractor) Flush() error {
	if len(extractor.writes) == 0 {
		return nil
	}

	probe := datura.Acquire("feature-extractor-flush", datura.APPJSON)
	defer probe.Release()

	if _, err := probe.Unpack(extractor.writes); err != nil {
		return err
	}

	extractor.writes = nil

	return nil
}

func (extractor *FeatureExtractor) Close() error {
	return extractor.Flush()
}

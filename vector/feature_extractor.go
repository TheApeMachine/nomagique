package vector

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/adaptive"
)

/*
FeatureExtractor is an atomic compute primitive.

The constructor artifact is config (attributes); its payload buses inbound wire
from Write to Read. Read unpacks the frame, extracts features, and emits output.
*/
type FeatureExtractor struct {
	artifact *datura.Artifact
	emas     map[string]*adaptive.EMA
	writes   []byte
}

/*
NewFeatureExtractor builds an extractor wired from schema attributes.
*/
func NewFeatureExtractor(artifact *datura.Artifact) *FeatureExtractor {
	return &FeatureExtractor{
		artifact: artifact,
		emas:     map[string]*adaptive.EMA{},
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

		var err error
		sample, err = extractor.transform(transform, role+":"+input, sample)
		if err != nil {
			return 0, err
		}

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

func (extractor *FeatureExtractor) transform(
	transform string,
	key string,
	sample float64,
) (float64, error) {
	if transform != "ema" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"feature-extractor: transform not registered",
			nil,
		))
	}

	ema, ok := extractor.emas[key]
	if !ok {
		ema = adaptive.NewEMA()
		extractor.emas[key] = ema
	}

	out, err := ema.Measure(sample)
	if err != nil {
		return 0, err
	}

	if math.IsNaN(out) || math.IsInf(out, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"feature-extractor: transform output non-finite",
			nil,
		))
	}

	return out, nil
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

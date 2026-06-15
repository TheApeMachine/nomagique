package vector

import (
	"fmt"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
FeatureFormula maps indexed input channels to one derived feature value.

Each formula receives the full input slice and returns a single scalar.
*/
type FeatureFormula func(inputs []float64) float64

/*
FeatureExtractor holds raw input channels and derived feature slots in reusable
buffers. Write brings an artifact in; Read updates one channel and refreshes features.
*/
type FeatureExtractor struct {
	artifact *datura.Artifact
	inputs   []float64
	features []float64
	formulas []FeatureFormula
	output   float64
}

/*
NewFeatureExtractor builds an extractor with inputCount channels and one formula
per derived feature, evaluated in registration order.
*/
func NewFeatureExtractor(inputCount int, formulas ...FeatureFormula) (*FeatureExtractor, error) {
	if inputCount <= 0 {
		return nil, errnie.Error(fmt.Errorf(
			"vector: NewFeatureExtractor inputCount must be positive, got %d",
			inputCount,
		))
	}

	if len(formulas) == 0 {
		return nil, errnie.Error(fmt.Errorf(
			"vector: NewFeatureExtractor requires at least one formula",
		))
	}

	return &FeatureExtractor{
		artifact: datura.Acquire("feature-extractor", datura.Artifact_Type_json),
		inputs:   make([]float64, inputCount),
		features: make([]float64, len(formulas)),
		formulas: formulas,
	}, nil
}

func (extractor *FeatureExtractor) Write(p []byte) (int, error) {
	return extractor.artifact.Write(p)
}

func (extractor *FeatureExtractor) Read(p []byte) (int, error) {
	values := float64Batch(extractor.artifact)

	if len(values) >= 2 {
		channel := int(values[0])

		if channel >= 0 && channel < len(extractor.inputs) {
			extractor.inputs[channel] = values[1]
			extractor.Extract()
			extractor.output = extractor.features[0]
		}
	}

	putFloat64Payload(&extractor.artifact, "extractor", extractor.output)

	return extractor.artifact.Read(p)
}

func (extractor *FeatureExtractor) Close() error {
	return nil
}

/*
InputCount returns the number of raw input channels.
*/
func (extractor *FeatureExtractor) InputCount() int {
	return len(extractor.inputs)
}

/*
SetInput updates one raw input channel by index.
*/
func (extractor *FeatureExtractor) SetInput(index int, value float64) error {
	if index < 0 || index >= len(extractor.inputs) {
		return errnie.Error(fmt.Errorf(
			"vector: FeatureExtractor.SetInput index %d outside [0,%d)",
			index,
			len(extractor.inputs),
		))
	}

	extractor.inputs[index] = value

	return nil
}

/*
Input returns the current value of one raw input channel.
*/
func (extractor *FeatureExtractor) Input(index int) (float64, error) {
	if index < 0 || index >= len(extractor.inputs) {
		return 0, errnie.Error(fmt.Errorf(
			"vector: FeatureExtractor.Input index %d outside [0,%d)",
			index,
			len(extractor.inputs),
		))
	}

	return extractor.inputs[index], nil
}

/*
Extract evaluates every formula over the current inputs into the feature buffer.
*/
func (extractor *FeatureExtractor) Extract() []float64 {
	for index, formula := range extractor.formulas {
		extractor.features[index] = formula(extractor.inputs)
	}

	return extractor.features
}

/*
FeatureCount returns the number of derived feature slots.
*/
func (extractor *FeatureExtractor) FeatureCount() int {
	return len(extractor.features)
}

/*
Feature reads one derived feature value by index after Extract.
*/
func (extractor *FeatureExtractor) Feature(index int) (float64, error) {
	if index < 0 || index >= len(extractor.features) {
		return 0, errnie.Error(fmt.Errorf(
			"vector: FeatureExtractor.Feature index %d outside [0,%d)",
			index,
			len(extractor.features),
		))
	}

	return extractor.features[index], nil
}

/*
Reset clears all input and feature buffers to zero.
*/
func (extractor *FeatureExtractor) Reset() error {
	for index := range extractor.inputs {
		extractor.inputs[index] = 0
	}

	for index := range extractor.features {
		extractor.features[index] = 0
	}

	extractor.output = 0

	return nil
}

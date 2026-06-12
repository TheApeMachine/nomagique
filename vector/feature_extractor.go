package vector

import (
	"fmt"

	"github.com/theapemachine/errnie"
)

/*
FeatureFormula maps indexed input channels to one derived feature value.
*/
type FeatureFormula func(inputs []float64) float64

/*
FeatureExtractor is a schema-agnostic vector-to-vector transformer.
It maps raw input channels to derived feature slots using registered formulas.
*/
type FeatureExtractor struct {
	inputs   []float64
	features []float64
	formulas []FeatureFormula
}

/*
NewFeatureExtractor instantiates a reusable extractor with preallocated buffers.
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
		inputs:   make([]float64, inputCount),
		features: make([]float64, len(formulas)),
		formulas: formulas,
	}, nil
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

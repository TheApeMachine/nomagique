package vector

import (
	"fmt"

	"github.com/theapemachine/errnie"
)

/*
FeatureSampleConfig selects one feature vector slot.
*/
type FeatureSampleConfig struct {
	FeatureIndex int
	OutputKey    string
}

/*
FeatureSampleOutput reports the selected named feature.
*/
type FeatureSampleOutput struct {
	Value  NamedValue
	Vector FeatureVector
}

/*
FeatureSample copies one feature vector slot into a named output.
*/
type FeatureSample struct {
	config FeatureSampleConfig
}

/*
NewFeatureSample returns a feature-index selector.
*/
func NewFeatureSample(config FeatureSampleConfig) *FeatureSample {
	return &FeatureSample{
		config: config,
	}
}

/*
Measure returns the selected feature as a named output.
*/
func (featureSample *FeatureSample) Measure(
	vector FeatureVector,
) (FeatureSampleOutput, error) {
	if featureSample.config.OutputKey == "" {
		return FeatureSampleOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"feature-sample: output key required",
			nil,
		))
	}

	if featureSample.config.FeatureIndex < 0 ||
		len(vector.Features) <= featureSample.config.FeatureIndex {
		return FeatureSampleOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			fmt.Sprintf(
				"feature-sample: feature index %d out of range for %d features",
				featureSample.config.FeatureIndex,
				len(vector.Features),
			),
			nil,
		))
	}

	sample := vector.Features[featureSample.config.FeatureIndex]

	if err := finiteVector("feature-sample", sample); err != nil {
		return FeatureSampleOutput{}, err
	}

	value := NamedValue{
		Name:  featureSample.config.OutputKey,
		Value: sample,
	}

	vector = vector.WithValue(value)

	return FeatureSampleOutput{
		Value:  value,
		Vector: vector,
	}, nil
}

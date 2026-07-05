package vector

import (
	"math"

	"github.com/theapemachine/errnie"
)

/*
SpreadSampleConfig describes a relative spread over named inputs.
*/
type SpreadSampleConfig struct {
	Inputs    []string
	OutputKey string
}

/*
SpreadSampleOutput reports the spread and updated feature vector.
*/
type SpreadSampleOutput struct {
	Value  NamedValue
	Vector FeatureVector
}

/*
SpreadSample derives a relative spread sample from named inputs.
*/
type SpreadSample struct {
	config SpreadSampleConfig
}

/*
NewSpreadSample returns a typed spread sample stage.
*/
func NewSpreadSample(config SpreadSampleConfig) *SpreadSample {
	return &SpreadSample{
		config: config,
	}
}

/*
Measure computes a relative spread and attaches it as a named output.
*/
func (spreadSample *SpreadSample) Measure(
	vector FeatureVector,
) (SpreadSampleOutput, error) {
	if len(spreadSample.config.Inputs) == 0 {
		return SpreadSampleOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"spread-sample: inputs required",
			nil,
		))
	}

	if spreadSample.config.OutputKey == "" {
		return SpreadSampleOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"spread-sample: output key required",
			nil,
		))
	}

	low := math.Inf(1)
	high := math.Inf(-1)

	for _, inputKey := range spreadSample.config.Inputs {
		data, exists := vector.Value(inputKey)

		if !exists {
			return SpreadSampleOutput{}, errnie.Error(errnie.Err(
				errnie.Validation,
				"spread-sample: input not found: "+inputKey,
				nil,
			))
		}

		if err := finiteVector("spread-sample: "+inputKey, data); err != nil {
			return SpreadSampleOutput{}, err
		}

		low = math.Min(low, data)
		high = math.Max(high, data)
	}

	mid := (low + high) / 2

	if mid <= 0 {
		return SpreadSampleOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"spread-sample: mid price must be positive",
			nil,
		))
	}

	value := NamedValue{
		Name:  spreadSample.config.OutputKey,
		Value: (high - low) / mid,
	}

	vector = vector.WithValue(value)

	return SpreadSampleOutput{
		Value:  value,
		Vector: vector,
	}, nil
}

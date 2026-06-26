package vector

import (
	"fmt"
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
FeatureSample copies one feature vector slot onto the sample field.
*/
type FeatureSample struct {
	config *datura.Artifact
}

/*
NewFeatureSample returns a feature-index selector configured on the artifact.
*/
func NewFeatureSample(config *datura.Artifact) *FeatureSample {

	return &FeatureSample{
		config: config,
	}
}

func (featureSample *FeatureSample) Read(payload []byte) (int, error) {
	state := datura.Acquire("feature-sample-state", datura.APPJSON)

	if _, err := state.Unpack(featureSample.config.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"feature-sample: state write failed",
			err,
		))
	}

	features := datura.Peek[[]float64](state, "features")
	featureIndex := int(datura.Peek[float64](featureSample.config, "featureIndex"))

	if featureIndex < 0 || len(features) <= featureIndex {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			fmt.Sprintf("feature-sample: feature index %d out of range for %d features", featureIndex, len(features)),
			nil,
		))
	}

	sample := features[featureIndex]

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"feature-sample: sample is non-finite",
			nil,
		))
	}

	state.MergeOutput("value", sample)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")

	return state.PackInto(payload)
}

func (featureSample *FeatureSample) Write(payload []byte) (int, error) {
	featureSample.config.WithPayload(payload)
	return len(payload), nil
}

func (featureSample *FeatureSample) Close() error {
	return nil
}

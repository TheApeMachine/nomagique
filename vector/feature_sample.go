package vector

import (
	"github.com/theapemachine/datura"
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

func (featureSample *FeatureSample) Write(payload []byte) (int, error) {
	featureSample.config.WithPayload(payload)
	return len(payload), nil
}

func (featureSample *FeatureSample) Read(payload []byte) (int, error) {
	state := datura.Acquire("feature-sample-state", datura.APPJSON)

	if _, err := state.Write(featureSample.config.DecryptPayload()); err != nil {
		return 0, err
	}

	features := datura.Peek[[]float64](state, "features")
	featureIndex := int(datura.Peek[float64](featureSample.config, "featureIndex"))

	if featureIndex < 0 || len(features) <= featureIndex {
		return state.Read(payload)
	}

	state.Poke(features[featureIndex], "sample")

	return state.Read(payload)
}

func (featureSample *FeatureSample) Close() error {
	return nil
}

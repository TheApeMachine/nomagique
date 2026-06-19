package vector

import (
	"github.com/theapemachine/datura"
)

/*
FeatureSample copies one feature vector slot onto the sample field.
*/
type FeatureSample struct {
	config *datura.Artifact
	staged *datura.Artifact
}

/*
NewFeatureSample returns a feature-index selector configured on the artifact.
*/
func NewFeatureSample(config *datura.Artifact) *FeatureSample {
	return &FeatureSample{
		config: config,
		staged: datura.Acquire("feature-sample", datura.APPJSON),
	}
}

func (featureSample *FeatureSample) Write(payload []byte) (int, error) {
	return featureSample.staged.Write(payload)
}

func (featureSample *FeatureSample) Read(payload []byte) (int, error) {
	features := datura.Peek[[]float64](featureSample.staged, "features")
	featureIndex := int(datura.Peek[float64](featureSample.config, "featureIndex"))

	if featureIndex < 0 || len(features) <= featureIndex {
		return featureSample.staged.Read(payload)
	}

	featureSample.staged.Poke(features[featureIndex], "sample")

	return featureSample.staged.Read(payload)
}

func (featureSample *FeatureSample) Close() error {
	return nil
}

package vector

import "github.com/theapemachine/nomagique/core"

/*
FeatureNode exposes one derived feature slot as a core.Number pipeline stage.
Call Extract on the shared extractor before Observe when inputs change.
*/
type FeatureNode struct {
	extractor  *FeatureExtractor
	featureIdx int
}

/*
NewFeatureNode binds one feature index on a shared extractor.
*/
func NewFeatureNode(extractor *FeatureExtractor, featureIndex int) *FeatureNode {
	return &FeatureNode{
		extractor:  extractor,
		featureIdx: featureIndex,
	}
}

/*
Observe returns the selected feature value from the extractor buffer.
*/
func (featureNode *FeatureNode) Observe(_ ...core.Number) core.Float64 {
	value, err := featureNode.extractor.Feature(featureNode.featureIdx)

	if err != nil {
		return 0
	}

	return core.Float64(value)
}

/*
Apply returns the selected feature without parsing pipeline inputs.
*/
func (featureNode *FeatureNode) Apply(_ core.Float64, _ []core.Float64) (core.Float64, error) {
	value, err := featureNode.extractor.Feature(featureNode.featureIdx)

	if err != nil {
		return 0, err
	}

	return core.Float64(value), nil
}

/*
Reset is a no-op; the extractor owns mutable state.
*/
func (featureNode *FeatureNode) Reset() error {
	return nil
}

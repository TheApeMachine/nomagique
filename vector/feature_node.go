package vector

import (
	"github.com/theapemachine/datura"
)

/*
FeatureNode exposes one derived feature from a shared FeatureExtractor as an
io.ReadWriteCloser pipeline stage.

Wire several InputSlots into the same extractor, then compose FeatureNode for
each derived feature inside nomagique.Number(...). Each Read re-runs Extract
so features always reflect the latest inputs.
*/
type FeatureNode struct {
	artifact   *datura.Artifact
	extractor  *FeatureExtractor
	featureIdx int
	output     float64
}

/*
NewFeatureNode binds one feature index on a shared extractor.
*/
func NewFeatureNode(
	extractor *FeatureExtractor, featureIndex int,
) *FeatureNode {
	if extractor == nil {
		return nil
	}

	if featureIndex < 0 || featureIndex >= extractor.FeatureCount() {
		return nil
	}

	return &FeatureNode{
		artifact:   datura.Acquire("feature-node", datura.Artifact_Type_json),
		extractor:  extractor,
		featureIdx: featureIndex,
	}
}

func (featureNode *FeatureNode) Write(p []byte) (int, error) {
	return featureNode.artifact.Write(p)
}

func (featureNode *FeatureNode) Read(p []byte) (int, error) {
	featureNode.refresh()
	putFloat64Payload(&featureNode.artifact, "feature-node", featureNode.output)

	return featureNode.artifact.Read(p)
}

func (featureNode *FeatureNode) Close() error {
	return nil
}

/*
Reset is a no-op; the shared extractor owns mutable state.
*/
func (featureNode *FeatureNode) Reset() error {
	return nil
}

func (featureNode *FeatureNode) refresh() {
	featureNode.extractor.Extract()

	value, err := featureNode.extractor.Feature(featureNode.featureIdx)

	if err != nil {
		return
	}

	featureNode.output = value
}

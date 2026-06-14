package vector

import (
	"fmt"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/core"
)

/*
FeatureNode exposes one derived feature from a shared FeatureExtractor as a
core.Number pipeline stage.

Wire several InputSlots into the same extractor (bid, ask, quantities), then
compose FeatureNode for mid, spread, or imbalance inside nomagique.Number(...).
Each Observe re-runs Extract so features always reflect the latest inputs.

FeatureNode also implements core.Stage for fast-path pipeline Apply.
*/
type FeatureNode struct {
	extractor  *FeatureExtractor
	featureIdx int
}

/*
NewFeatureNode binds one feature index on a shared extractor.
*/
func NewFeatureNode(extractor *FeatureExtractor, featureIndex int) (*FeatureNode, error) {
	if extractor == nil {
		return nil, errnie.Error(fmt.Errorf("vector: NewFeatureNode requires extractor"))
	}

	if featureIndex < 0 || featureIndex >= extractor.FeatureCount() {
		return nil, errnie.Error(fmt.Errorf(
			"vector: NewFeatureNode featureIndex %d outside [0,%d)",
			featureIndex,
			extractor.FeatureCount(),
		))
	}

	return &FeatureNode{
		extractor:  extractor,
		featureIdx: featureIndex,
	}, nil
}

/*
Observe runs Extract on the shared extractor and returns the selected feature.
*/
func (featureNode *FeatureNode) Observe(_ ...core.Number) core.Float64 {
	featureNode.extractor.Extract()

	value, err := featureNode.extractor.Feature(featureNode.featureIdx)

	if err != nil {
		return 0
	}

	return core.Float64(value)
}

/*
Apply runs Extract and returns the selected feature without parsing pipeline inputs.
*/
func (featureNode *FeatureNode) Apply(_ core.Float64, _ []core.Float64) (core.Float64, error) {
	featureNode.extractor.Extract()

	value, err := featureNode.extractor.Feature(featureNode.featureIdx)

	if err != nil {
		return 0, err
	}

	return core.Float64(value), nil
}

/*
Reset is a no-op; the shared extractor owns mutable state.
*/
func (featureNode *FeatureNode) Reset() error {
	return nil
}

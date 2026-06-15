package vector

import (
	"fmt"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/core"
)

/*
FeatureNode exposes one derived feature from a shared FeatureExtractor as a
core.Number pipeline stage.

Wire several InputSlots into the same extractor, then compose FeatureNode for
each derived feature inside nomagique.Number(...). Each Observe re-runs Extract
so features always reflect the latest inputs.
*/
type FeatureNode[T ~float64] struct {
	extractor  *FeatureExtractor
	featureIdx int
	output     core.Scalar[T]
}

/*
NewFeatureNode binds one feature index on a shared extractor.
*/
func NewFeatureNode[T ~float64](
	extractor *FeatureExtractor, featureIndex int,
) (*FeatureNode[T], error) {
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

	return &FeatureNode[T]{
		extractor:  extractor,
		featureIdx: featureIndex,
	}, nil
}

/*
Observe runs Extract on the shared extractor and returns the selected feature.
*/
func (featureNode *FeatureNode[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	featureNode.refresh()

	return featureNode.output
}

/*
Reset is a no-op; the shared extractor owns mutable state.
*/
func (featureNode *FeatureNode[T]) Reset() error {
	return nil
}

func (featureNode *FeatureNode[T]) refresh() {
	featureNode.extractor.Extract()

	value, err := featureNode.extractor.Feature(featureNode.featureIdx)

	if err != nil {
		return
	}

	featureNode.output = core.Scalar[T](T(value))
}

package vector

import (
	"fmt"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/core"
)

/*
InputSlot binds one raw input channel on a shared FeatureExtractor to the
nomagique pipeline.

Observe writes the boundary sample into that channel. Multiple InputSlots can
share one extractor, one per channel.
*/
type InputSlot[T ~float64] struct {
	extractor *FeatureExtractor
	channel   int
	output    core.Scalar[T]
}

/*
NewInputSlot binds channel on extractor for composable input updates.
*/
func NewInputSlot[T ~float64](
	extractor *FeatureExtractor, channel int,
) (*InputSlot[T], error) {
	if extractor == nil {
		return nil, errnie.Error(fmt.Errorf("vector: NewInputSlot requires extractor"))
	}

	if channel < 0 || channel >= extractor.InputCount() {
		return nil, errnie.Error(fmt.Errorf(
			"vector: NewInputSlot channel %d outside [0,%d)",
			channel,
			extractor.InputCount(),
		))
	}

	return &InputSlot[T]{
		extractor: extractor,
		channel:   channel,
	}, nil
}

/*
Observe stores the carried sample into the bound input channel.
*/
func (inputSlot *InputSlot[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if len(inputs) == 0 {
		return inputSlot.output
	}

	sample, ok := inputs[0].(core.Scalar[T])

	if !ok {
		return inputSlot.output
	}

	if len(inputs) > 1 {
		if work, workOK := inputs[1].(core.Scalar[T]); workOK {
			sample = core.Scalar[T](T(sample) + T(work))
		}
	}

	if err := inputSlot.extractor.SetInput(inputSlot.channel, float64(sample)); err != nil {
		return inputSlot.output
	}

	inputSlot.output = sample

	return inputSlot.output
}

/*
Reset is a no-op; the shared extractor owns buffer state.
*/
func (inputSlot *InputSlot[T]) Reset() error {
	return nil
}

package vector

import (
	"fmt"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/core"
)

/*
InputSlot binds one raw input channel on a shared FeatureExtractor to the
nomagique pipeline.

Observe writes the boundary sample into that channel — use when composing live
feeds into an extractor without calling SetInput from adapter code. When
observed through a nomagique.Number pipeline, the carried boundary sample is the
last input after pipeline wiring.

InputSlot implements core.Number. Multiple InputSlots can share one extractor,
one per channel (bid price, ask price, and so on).
*/
type InputSlot struct {
	extractor *FeatureExtractor
	channel   int
}

/*
NewInputSlot binds channel on extractor for composable input updates.
*/
func NewInputSlot(extractor *FeatureExtractor, channel int) (*InputSlot, error) {
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

	return &InputSlot{
		extractor: extractor,
		channel:   channel,
	}, nil
}

/*
Observe stores the sample into the bound input channel.

Empty input returns zero and leaves the channel unchanged.
*/
func (inputSlot *InputSlot) Observe(inputs ...core.Number) core.Float64 {
	samples := nomagique.Samples(core.Numbers(inputs))

	if len(samples) == 0 {
		return 0
	}

	value := samples[0]

	if len(samples) > 1 {
		value = samples[len(samples)-1]
	}

	if err := inputSlot.extractor.SetInput(inputSlot.channel, value); err != nil {
		return 0
	}

	return core.Float64(value)
}

/*
Reset is a no-op; the shared extractor owns buffer state.
*/
func (inputSlot *InputSlot) Reset() error {
	return nil
}

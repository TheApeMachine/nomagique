package vector

import (
	"github.com/theapemachine/datura"
)

/*
InputSlot binds one raw input channel on a shared FeatureExtractor to the
nomagique pipeline.

Write stores the boundary sample into that channel. Multiple InputSlots can
share one extractor, one per channel.
*/
type InputSlot struct {
	artifact  *datura.Artifact
	extractor *FeatureExtractor
	channel   int
	output    float64
}

/*
NewInputSlot binds channel on extractor for composable input updates.
*/
func NewInputSlot(
	extractor *FeatureExtractor, channel int,
) *InputSlot {
	if extractor == nil {
		return nil
	}

	if channel < 0 || channel >= extractor.InputCount() {
		return nil
	}

	return &InputSlot{
		artifact:  datura.Acquire("input-slot", datura.Artifact_Type_json),
		extractor: extractor,
		channel:   channel,
	}
}

func (inputSlot *InputSlot) Write(p []byte) (int, error) {
	return inputSlot.artifact.Write(p)
}

func (inputSlot *InputSlot) Read(p []byte) (int, error) {
	sample, sampleOK := boundaryFloat64(inputSlot.artifact)

	if sampleOK {
		_ = inputSlot.extractor.SetInput(inputSlot.channel, sample)
		inputSlot.output = sample
		putFloat64Payload(&inputSlot.artifact, "input-slot", inputSlot.output)
	}

	return inputSlot.artifact.Read(p)
}

func (inputSlot *InputSlot) Close() error {
	return nil
}

/*
Reset is a no-op; the shared extractor owns mutable state.
*/
func (inputSlot *InputSlot) Reset() error {
	return nil
}

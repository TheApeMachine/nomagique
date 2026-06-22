package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Min tracks the smallest streamed sample.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Min struct {
	artifact *datura.Artifact
}

/*
NewMin returns a min stage wired from config attributes on the artifact.
*/
func NewMin(artifact *datura.Artifact) *Min {
	artifact.Inspect("statistic", "min", "NewMin()")

	return &Min{
		artifact: artifact,
	}
}

func (min *Min) Write(payload []byte) (int, error) {
	min.artifact.WithPayload(payload)
	return len(payload), nil
}

func (min *Min) Read(payload []byte) (int, error) {
	state := datura.Acquire("min-state", datura.APPJSON)
	state.Inspect("statistic", "min", "Read()", "p")

	if _, err := state.Write(min.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	sampleKey := WireInputKey(min.artifact, state, "sample")
	sample, err := WireScalar(min.artifact, state, sampleKey)

	if err != nil {
		return 0, err
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"min: sample is non-finite",
			nil,
		))
	}

	count := datura.Peek[float64](min.artifact, "output", "count")
	value := datura.Peek[float64](min.artifact, "output", "value")

	if count == 0 {
		value = sample
	}

	if count > 0 && sample < value {
		value = sample
	}

	count++
	min.artifact.Poke(count, "output", "count")
	min.artifact.Poke(value, "output", "value")
	state.MergeOutput("value", value)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(payload)
}

func (min *Min) Close() error {
	return nil
}

package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Max tracks the largest streamed sample.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Max struct {
	artifact *datura.Artifact
}

/*
NewMax returns a max stage wired from config attributes on the artifact.
*/
func NewMax(artifact *datura.Artifact) *Max {
	artifact.Inspect("statistic", "max", "NewMax()")

	return &Max{
		artifact: artifact,
	}
}

func (max *Max) Write(payload []byte) (int, error) {
	max.artifact.WithPayload(payload)
	return len(payload), nil
}

func (max *Max) Read(payload []byte) (int, error) {
	state := datura.Acquire("max-state", datura.APPJSON)
	state.Inspect("statistic", "max", "Read()", "p")

	if _, err := state.Write(max.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	sampleKey := WireInputKey(max.artifact, state, "sample")
	sample, err := WireScalar(max.artifact, state, sampleKey)

	if err != nil {
		return 0, err
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"max: sample is non-finite",
			nil,
		))
	}

	count := datura.Peek[float64](max.artifact, "output", "count")
	value := datura.Peek[float64](max.artifact, "output", "value")

	if count == 0 {
		value = sample
	}

	if count > 0 && sample > value {
		value = sample
	}

	count++
	max.artifact.Poke(count, "output", "count")
	max.artifact.Poke(value, "output", "value")
	state.MergeOutput("value", value)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(payload)
}

func (max *Max) Close() error {
	return nil
}

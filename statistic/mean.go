package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Mean computes a running arithmetic mean of streamed samples.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Mean struct {
	artifact *datura.Artifact
}

/*
NewMean returns a mean stage wired from config attributes on the artifact.
*/
func NewMean(artifact *datura.Artifact) *Mean {
	artifact.Inspect("statistic", "mean", "NewMean()")

	return &Mean{
		artifact: artifact,
	}
}

func (mean *Mean) Write(payload []byte) (int, error) {
	mean.artifact.WithPayload(payload)
	return len(payload), nil
}

func (mean *Mean) Read(payload []byte) (int, error) {
	state := datura.Acquire("mean-state", datura.APPJSON)
	state.Inspect("statistic", "mean", "Read()", "p")

	if _, err := state.Write(mean.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	sample := datura.Peek[float64](state, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"mean: sample is non-finite",
			nil,
		))
	}

	count := datura.Peek[float64](mean.artifact, "output", "count")
	sum := datura.Peek[float64](mean.artifact, "output", "sum")

	count++
	sum += sample

	value := sum / count

	mean.artifact.Poke(count, "output", "count")
	mean.artifact.Poke(sum, "output", "sum")
	mean.artifact.Poke(value, "output", "value")
	state.MergeOutput("value", value)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(payload)
}

func (mean *Mean) Close() error {
	return nil
}

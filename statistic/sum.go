package statistic

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
Sum integrates streamed samples into a running total.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Sum struct {
	artifact *datura.Artifact
}

/*
NewSum returns a sum stage wired from config attributes on the artifact.
*/
func NewSum(artifact *datura.Artifact) *Sum {
	artifact.Inspect("statistic", "sum", "NewSum()")

	return &Sum{
		artifact: artifact,
	}
}

func (sum *Sum) Write(payload []byte) (int, error) {
	sum.artifact.WithPayload(payload)
	return len(payload), nil
}

func (sum *Sum) Read(payload []byte) (int, error) {
	state := datura.Acquire("sum-state", datura.APPJSON)
	state.Inspect("statistic", "sum", "Read()", "p")

	if _, err := state.Write(sum.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	sample := datura.Peek[float64](state, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return state.Read(payload)
	}

	value := datura.Peek[float64](sum.artifact, "output", "value") + sample

	sum.artifact.Poke(value, "output", "value")
	state.MergeOutput("value", value)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(payload)
}

func (sum *Sum) Close() error {
	return nil
}

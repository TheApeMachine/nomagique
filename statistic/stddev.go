package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
StdDev computes the sample standard deviation over retained history.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type StdDev struct {
	artifact *datura.Artifact
}

/*
NewStdDev returns a standard-deviation stage wired from config attributes on the artifact.
*/
func NewStdDev(artifact *datura.Artifact) *StdDev {
	return &StdDev{
		artifact: artifact,
	}
}

func (stdDev *StdDev) Read(payload []byte) (int, error) {
	state := datura.Acquire("stddev-state", datura.APPJSON)

	if _, err := state.Write(stdDev.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	state.Inspect("statistic", "stddev", "Read()", "p")

	sample := datura.Peek[float64](state, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"stddev: sample is non-finite",
			nil,
		))
	}

	history := datura.Peek[[]float64](stdDev.artifact, "history")
	history = append(history, sample)
	stdDev.artifact.Poke(history, "history")

	if len(history) < 2 {
		return 0, nil
	}

	value := stat.StdDev(history, nil)
	state.MergeOutput("value", value)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")
	return state.Read(payload)
}

func (stdDev *StdDev) Write(payload []byte) (int, error) {
	stdDev.artifact.WithPayload(payload)
	return len(payload), nil
}

func (stdDev *StdDev) Close() error {
	return nil
}

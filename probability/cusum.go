package probability

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
CUSUM accumulates sequential change evidence from a sample stream.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type CUSUM struct {
	artifact *datura.Artifact
}

/*
NewCUSUM returns a change-detection stage wired from config attributes on the artifact.
*/
func NewCUSUM(artifact *datura.Artifact) *CUSUM {
	artifact.Inspect("probability", "cusum", "NewCUSUM()")

	return &CUSUM{
		artifact: artifact,
	}
}

func (cusum *CUSUM) Write(payload []byte) (int, error) {
	cusum.artifact.WithPayload(payload)
	return len(payload), nil
}

func (cusum *CUSUM) Read(payload []byte) (int, error) {
	state := datura.Acquire("cusum-state", datura.APPJSON)
	state.Inspect("probability", "cusum", "Read()", "p")

	if _, err := state.Write(cusum.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	if datura.Peek[float64](state, "reset") != 0 {
		cusum.artifact.WithAttributes(datura.Map[any]{})
		state.MergeOutput("ready", 0)
		state.MergeOutput("value", 0)
		state.Merge("root", "output")
		state.Merge("inputs", []string{"value"})
		return state.Read(payload)
	}

	if !attributeKeyPresent(state, "sample") {
		return state.Read(payload)
	}

	sample := datura.Peek[float64](state, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return state.Read(payload)
	}

	cusumState := CUSUMState{
		Target:   datura.Peek[float64](cusum.artifact, "output", "target"),
		Positive: datura.Peek[float64](cusum.artifact, "output", "positive"),
		Prev:     datura.Peek[float64](cusum.artifact, "output", "prev"),
		Min:      datura.Peek[float64](cusum.artifact, "output", "min"),
		Max:      datura.Peek[float64](cusum.artifact, "output", "max"),
		Rate:     datura.Peek[float64](cusum.artifact, "output", "rate"),
		Ready:    datura.Peek[float64](cusum.artifact, "output", "ready") != 0,
	}

	value := ObserveCUSUM(&cusumState, sample)

	ready := 0.0

	if cusumState.Ready {
		ready = 1
	}

	cusum.artifact.Poke(cusumState.Target, "output", "target")
	cusum.artifact.Poke(cusumState.Positive, "output", "positive")
	cusum.artifact.Poke(cusumState.Prev, "output", "prev")
	cusum.artifact.Poke(cusumState.Min, "output", "min")
	cusum.artifact.Poke(cusumState.Max, "output", "max")
	cusum.artifact.Poke(cusumState.Rate, "output", "rate")
	cusum.artifact.Poke(ready, "output", "ready")
	cusum.artifact.Poke(value, "output", "value")
	state.MergeOutput("value", value)
	state.MergeOutput("ready", ready)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(payload)
}

func (cusum *CUSUM) Close() error {
	return nil
}

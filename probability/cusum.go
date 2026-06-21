package probability

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
CUSUM accumulates sequential change evidence from a sample stream.
*/
type CUSUM struct {
	artifact *datura.Artifact
}

/*
NewCUSUM returns a change-detection stage ready from its first observation.
*/
func NewCUSUM() *CUSUM {
	return &CUSUM{
		artifact: datura.Acquire("cusum", datura.APPJSON),
	}
}

func (cusum *CUSUM) Write(p []byte) (int, error) {
	return cusum.artifact.Write(p)
}

func (cusum *CUSUM) Read(p []byte) (int, error) {
	if !attributeKeyPresent(cusum.artifact, "sample") {
		return cusum.artifact.Read(p)
	}

	sample := datura.Peek[float64](cusum.artifact, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return cusum.artifact.Read(p)
	}

	output := datura.Peek[datura.Map[float64]](cusum.artifact, "output")
	state := CUSUMState{}

	if output != nil {
		state.Target = output["target"]
		state.Positive = output["positive"]
		state.Prev = output["prev"]
		state.Min = output["min"]
		state.Max = output["max"]
		state.Rate = output["rate"]
		state.Ready = output["ready"] != 0
	}

	value := ObserveCUSUM(&state, sample)

	ready := 0.0

	if state.Ready {
		ready = 1
	}

	cusum.artifact.Poke(datura.Map[float64]{
		"target":   state.Target,
		"positive": state.Positive,
		"prev":     state.Prev,
		"min":      state.Min,
		"max":      state.Max,
		"rate":     state.Rate,
		"ready":    ready,
		"value":    value,
	}, "output")

	return cusum.artifact.Read(p)
}

func (cusum *CUSUM) Close() error {
	return nil
}

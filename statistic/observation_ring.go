package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
ObservationRing retains recent scalar observations with capacity derived from
sample span rather than a fixed magic bound.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type ObservationRing struct {
	artifact *datura.Artifact
}

/*
NewObservationRing returns an observation ring stage wired from config attributes on the artifact.
*/
func NewObservationRing(artifact *datura.Artifact) *ObservationRing {
	artifact.Inspect("statistic", "observation-ring", "NewObservationRing()")

	return &ObservationRing{
		artifact: artifact,
	}
}

func (ring *ObservationRing) Write(payload []byte) (int, error) {
	ring.artifact.WithPayload(payload)
	return len(payload), nil
}

func (ring *ObservationRing) Read(payload []byte) (int, error) {
	state := datura.Acquire("observation-ring-state", datura.APPJSON)
	state.Inspect("statistic", "observation-ring", "Read()", "p")

	if _, err := state.Write(ring.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	sample := datura.Peek[float64](state, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"observation-ring: sample is non-finite",
			nil,
		))
	}

	if sample <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"observation-ring: sample must be positive",
			nil,
		))
	}

	history := datura.Peek[[]float64](ring.artifact, "history")
	history = append(history, sample)

	capacity := ring.capacityFor(history)

	if capacity > 0 && len(history) > capacity {
		history = history[len(history)-capacity:]
	}

	ring.artifact.Poke(history, "history")
	state.MergeOutput("value", sample)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(payload)
}

func (ring *ObservationRing) Close() error {
	return nil
}

func (ring *ObservationRing) capacityFor(values []float64) int {
	if len(values) == 0 {
		return 1
	}

	span := SpanOf(values)

	if span <= 0 {
		return len(values) + 1
	}

	return int(span) + 1
}

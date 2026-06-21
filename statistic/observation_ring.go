package statistic

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
ObservationRing retains recent scalar observations with capacity derived from
sample span rather than a fixed magic bound.
*/
type ObservationRing struct {
	artifact *datura.Artifact
}

func NewObservationRing() *ObservationRing {
	return &ObservationRing{
		artifact: datura.Acquire("observation_ring", datura.APPJSON),
	}
}

func (ring *ObservationRing) Write(p []byte) (int, error) {
	return ring.artifact.Write(p)
}

func (ring *ObservationRing) Read(p []byte) (int, error) {
	sample := datura.Peek[float64](ring.artifact, "sample")

	if sample <= 0 || math.IsNaN(sample) || math.IsInf(sample, 0) {
		return ring.artifact.Read(p)
	}

	history := datura.Peek[[]float64](ring.artifact, "history")
	history = append(history, sample)

	capacity := ring.capacityFor(history)

	if capacity > 0 && len(history) > capacity {
		history = history[len(history)-capacity:]
	}

	ring.artifact.Poke(history, "history")

	ring.artifact.Poke(datura.Map[float64]{"value": sample}, "output")

	return ring.artifact.Read(p)
}

func (ring *ObservationRing) Close() error {
	return nil
}

func (ring *ObservationRing) capacityFor(values []float64) int {
	if len(values) == 0 {
		return 1
	}

	if len(values) < 3 {
		return len(values) + 1
	}

	span := SpanOf(values)

	if span <= 0 {
		return 3
	}

	return int(span) + 1
}

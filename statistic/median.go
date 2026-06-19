package statistic

import (
	"math"
	"sort"

	"github.com/theapemachine/datura"
)

/*
Median computes the sample median over retained history or panel peers.
*/
type Median struct {
	artifact *datura.Artifact
}

/*
NewMedian creates a median stage.
*/
func NewMedian() *Median {
	return &Median{
		artifact: datura.Acquire("median", datura.APPJSON),
	}
}

func (median *Median) Read(p []byte) (int, error) {
	peers := datura.Peek[map[string]float64](median.artifact, "peers")

	if len(peers) > 0 {
		member := datura.Peek[float64](median.artifact, "member")
		peerSamples := make([]float64, 0, len(peers))

		for memberLabel, peerSample := range peers {
			if memberLabel == memberKey(member) {
				continue
			}

			peerSamples = append(peerSamples, peerSample)
		}

		value := 0.0

		if len(peerSamples) > 0 {
			value = MedianOf(peerSamples)
		}

		median.artifact.Poke(datura.Map[float64]{"value": value}, "output")

		return median.artifact.Read(p)
	}

	sample := datura.Peek[float64](median.artifact, "sample")

	if datura.Peek[float64](median.artifact, "non_finite") != 0 {
		median.artifact.Poke(datura.Map[float64]{"value": math.NaN()}, "output")

		return median.artifact.Read(p)
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		median.artifact.Poke(float64(1), "non_finite")
		median.artifact.Poke(datura.Map[float64]{"value": sample}, "output")

		return median.artifact.Read(p)
	}

	history := datura.Peek[[]float64](median.artifact, "history")
	history = append(history, sample)
	median.artifact.Poke(history, "history")

	median.artifact.Poke(datura.Map[float64]{"value": MedianOf(history)}, "output")

	return median.artifact.Read(p)
}

func (median *Median) Write(p []byte) (int, error) {
	return median.artifact.Write(p)
}

func (median *Median) Close() error {
	return nil
}

/*
MedianOf returns the median of values without weights.
*/
func MedianOf(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return math.NaN()
		}
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	middle := len(sorted) / 2

	if len(sorted)%2 == 1 {
		return sorted[middle]
	}

	return (sorted[middle-1] + sorted[middle]) / 2
}

package statistic

import (
	"math"
	"sort"

	"github.com/theapemachine/datura"
)

/*
Median computes the sample median over retained history or panel peers.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Median struct {
	artifact *datura.Artifact
}

/*
NewMedian returns a median stage wired from config attributes on the artifact.
*/
func NewMedian(artifact *datura.Artifact) *Median {
	artifact.Inspect("statistic", "median", "NewMedian()")

	return &Median{
		artifact: artifact,
	}
}

func (median *Median) Write(payload []byte) (int, error) {
	median.artifact.WithPayload(payload)
	return len(payload), nil
}

func (median *Median) Read(payload []byte) (int, error) {
	state := datura.Acquire("median-state", datura.APPJSON)
	state.Inspect("statistic", "median", "Read()", "p")

	if _, err := state.Write(median.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	peers := panelPeers(state)

	if len(peers) == 0 {
		peers = panelPeers(median.artifact)
	}

	if len(peers) > 0 {
		member := datura.Peek[float64](state, "member")
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

		state.MergeOutput("value", value)
		state.Merge("root", "output")
		state.Merge("inputs", []string{"value"})
		return state.Read(payload)
	}

	sample := datura.Peek[float64](state, "sample")

	if datura.Peek[float64](median.artifact, "non_finite") != 0 {
		state.MergeOutput("value", 0)
		state.Merge("root", "output")
		state.Merge("inputs", []string{"value"})
		return state.Read(payload)
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		median.artifact.Poke(float64(1), "non_finite")
		state.MergeOutput("value", 0)
		state.Merge("root", "output")
		state.Merge("inputs", []string{"value"})
		return state.Read(payload)
	}

	history := datura.Peek[[]float64](median.artifact, "history")
	history = append(history, sample)
	median.artifact.Poke(history, "history")

	state.MergeOutput("value", MedianOf(history))
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(payload)
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

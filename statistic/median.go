package statistic

import (
	"math"
	"sort"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
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

		if len(peerSamples) > 0 {
			value, ok := MedianOf(peerSamples)

			if !ok {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"median: peer samples are invalid",
					nil,
				))
			}

			state.MergeOutput("value", value)
			state.Merge("root", "output")
			state.Merge("inputs", []string{"value"})
			return state.Read(payload)
		}
	}

	sample := datura.Peek[float64](state, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"median: sample is non-finite",
			nil,
		))
	}

	history := datura.Peek[[]float64](median.artifact, "history")
	history = append(history, sample)
	median.artifact.Poke(history, "history")

	value, ok := MedianOf(history)

	if !ok {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"median: history is invalid",
			nil,
		))
	}

	state.MergeOutput("value", value)
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
func MedianOf(values []float64) (float64, bool) {
	if len(values) == 0 {
		return 0, false
	}

	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, false
		}
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	middle := len(sorted) / 2

	if len(sorted)%2 == 1 {
		return sorted[middle], true
	}

	return (sorted[middle-1] + sorted[middle]) / 2, true
}

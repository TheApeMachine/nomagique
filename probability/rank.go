package probability

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Rank tracks P(history <= current sample) over a span-derived window.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Rank struct {
	artifact *datura.Artifact
}

/*
NewRank returns an empirical rank probability stage wired from config attributes on the artifact.
*/
func NewRank(artifact *datura.Artifact) *Rank {
	return &Rank{
		artifact: artifact,
	}
}

func (rank *Rank) Read(payload []byte) (int, error) {
	state := datura.Acquire("rank-state", datura.APPJSON)

	if _, err := state.Write(rank.artifact.DecryptPayload()); err != nil {
		state.Release()

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"rank: state write failed",
			err,
		))
	}

	defer state.Release()

	if datura.Peek[float64](state, "reset") != 0 {
		rank.artifact.Poke([]float64{}, "history")
		rank.artifact.Poke(0.0, "output", "prev")
		rank.artifact.Poke(0.0, "output", "min")
		rank.artifact.Poke(0.0, "output", "max")
		rank.artifact.Poke(0.0, "output", "head")
		rank.artifact.Poke(0.0, "output", "count")
		rank.artifact.Poke(0.0, "output", "ready")
		rank.artifact.Poke(0.0, "output", "value")
		state.MergeOutput("ready", 0)
		state.MergeOutput("value", 0)
		state.Poke("output", "root")
		state.Poke([]string{"value"}, "inputs")
		return state.Read(payload)
	}

	sampleKey := datura.Peek[string](rank.artifact, "sampleKey")

	if sampleKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"rank: sampleKey required",
			nil,
		))
	}

	wireRoot := datura.Peek[string](state, "root")
	
	if wireRoot == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"rank: root required",
			nil,
		))
	}
	
	wireInputs := datura.Peek[[]string](state, "inputs")
	
	if len(wireInputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"rank: inputs required",
			nil,
		))
	}
	
	var sample float64
	sampleFound := false
	
	for wireIndex, wireInput := range wireInputs {
		if wireInput != sampleKey {
			continue
		}
	
		if wireRoot == "features" {
			features := datura.Peek[[]float64](state, wireRoot)
	
			if wireIndex >= len(features) {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"rank: feature index out of range",
					nil,
				))
			}
	
			sample = features[wireIndex]
		}
	
		if wireRoot != "features" {
			sample = datura.Peek[float64](state, wireRoot, wireInput)
		}
	
		sampleFound = true
	}
	
	if !sampleFound {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"rank: input not in inputs",
			nil,
		))
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"rank: sample is non-finite",
			nil,
		))
	}

	rankState := RankState{
		History: datura.Peek[[]float64](rank.artifact, "history"),
		Prev:    datura.Peek[float64](rank.artifact, "output", "prev"),
		Min:     datura.Peek[float64](rank.artifact, "output", "min"),
		Max:     datura.Peek[float64](rank.artifact, "output", "max"),
		Head:    int(datura.Peek[float64](rank.artifact, "output", "head")),
		Count:   int(datura.Peek[float64](rank.artifact, "output", "count")),
		Ready:   datura.Peek[float64](rank.artifact, "output", "ready") != 0,
	}

	value := ObserveRank(&rankState, sample)

	ready := 0.0

	if rankState.Ready {
		ready = 1
	}

	rank.artifact.Poke(rankState.History, "history")
	rank.artifact.Poke(rankState.Prev, "output", "prev")
	rank.artifact.Poke(rankState.Min, "output", "min")
	rank.artifact.Poke(rankState.Max, "output", "max")
	rank.artifact.Poke(float64(rankState.Head), "output", "head")
	rank.artifact.Poke(float64(rankState.Count), "output", "count")
	rank.artifact.Poke(ready, "output", "ready")
	rank.artifact.Poke(value, "output", "value")
	state.MergeOutput("value", value)
	state.MergeOutput("ready", ready)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")
	return state.Read(payload)
}

func (rank *Rank) Write(payload []byte) (int, error) {
	rank.artifact.WithPayload(payload)
	return len(payload), nil
}

func (rank *Rank) Close() error {
	return nil
}

/*
RankState tracks the empirical probability that observations fall at or below the current sample.
*/
type RankState struct {
	Prev    float64
	Min     float64
	Max     float64
	Head    int
	Count   int
	History []float64
	Ready   bool
}

func (state *RankState) Observe(sample float64) float64 {
	return ObserveRank(state, sample)
}

func (state *RankState) ObserveSamples(samples []float64, out []float64) {
	for index, sample := range samples {
		out[index] = ObserveRank(state, sample)
	}
}

func (state *RankState) Reset() {
	state.Prev = 0
	state.Min = 0
	state.Max = 0
	state.Head = 0
	state.Count = 0
	state.History = nil
	state.Ready = false
}

func ObserveRank(state *RankState, sample float64) float64 {
	if !state.Ready {
		state.Min = sample
		state.Max = sample
		state.Prev = sample
		state.Ready = true
		state.History = make([]float64, rankCapacity(0)+1)
		state.History[0] = sample
		state.Head = 0
		state.Count = 1

		return 1
	}

	return observeRankReady(state, sample)
}

func observeRankReady(state *RankState, sample float64) float64 {
	state.Min = math.Min(state.Min, sample)
	state.Max = math.Max(state.Max, sample)

	span := state.Max - state.Min
	state.ensureCapacity(rankCapacity(span))
	state.pushHistory(sample)
	state.Prev = sample

	return empiricalRank(state, sample)
}

func rankCapacity(span float64) int {
	return max(1, int(span)+1)
}

func (state *RankState) ensureCapacity(capacity int) {
	if len(state.History) >= capacity {
		return
	}

	next := make([]float64, capacity)
	copy(next, state.History)

	if state.Count > 0 {
		for index := 0; index < state.Count; index++ {
			source := (state.Head - index + len(state.History)) % len(state.History)
			next[index] = state.History[source]
		}

		state.Head = state.Count - 1
	}

	state.History = next
}

func (state *RankState) pushHistory(sample float64) {
	if len(state.History) == 0 {
		return
	}

	state.Head = (state.Head + 1) % len(state.History)
	state.History[state.Head] = sample

	if state.Count < len(state.History) {
		state.Count++
	}
}

func empiricalRank(state *RankState, sample float64) float64 {
	if state.Count == 0 {
		return 0
	}

	atOrBelow := 0

	for index := 0; index < state.Count; index++ {
		historyIndex := (state.Head - index + len(state.History)) % len(state.History)

		if state.History[historyIndex] <= sample {
			atOrBelow++
		}
	}

	return float64(atOrBelow) / float64(state.Count)
}

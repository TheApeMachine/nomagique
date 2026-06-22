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

func (rank *Rank) Write(payload []byte) (int, error) {
	rank.artifact.WithPayload(payload)
	return len(payload), nil
}

func (rank *Rank) Read(payload []byte) (int, error) {
	state := datura.Acquire("rank-state", datura.APPJSON)

	if _, err := state.Write(rank.artifact.DecryptPayload()); err != nil {
		state.Release()

		return 0, err
	}

	defer state.Release()

	if datura.Peek[float64](state, "reset") != 0 {
		rank.artifact.WithAttributes(datura.Map[any]{})
		state.MergeOutput("ready", 0)
		state.MergeOutput("value", 0)
		state.Merge("root", "output")
		state.Merge("inputs", []string{"value"})
		return state.Read(payload)
	}

	if !attributeKeyPresent(state, "sample") {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"rank: sample required",
			nil,
		))
	}

	sample := datura.Peek[float64](state, "sample")

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
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(payload)
}

func (rank *Rank) Close() error {
	return nil
}

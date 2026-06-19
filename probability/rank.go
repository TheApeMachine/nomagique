package probability

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
Rank tracks P(history <= current sample) over a span-derived window.
*/
type Rank struct {
	artifact *datura.Artifact
}

/*
NewRank returns an empirical rank probability stage ready from its first observation.
*/
func NewRank() *Rank {
	return &Rank{
		artifact: datura.Acquire("rank", datura.APPJSON).RetainStageAttributes(),
	}
}

func (rank *Rank) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](rank.artifact, "output") == nil

	rank.artifact.Clear("sample")

	n, err := rank.artifact.Write(p)

	if bootstrap {
		rank.artifact.Clear("output")
	}

	return n, err
}

func (rank *Rank) Read(p []byte) (int, error) {
	if !attributeKeyPresent(rank.artifact, "sample") {
		return rank.artifact.Read(p)
	}

	sample := datura.Peek[float64](rank.artifact, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return rank.artifact.Read(p)
	}

	output := datura.Peek[datura.Map[float64]](rank.artifact, "output")
	state := RankState{
		History: datura.Peek[[]float64](rank.artifact, "history"),
	}

	if output != nil {
		state.Prev = output["prev"]
		state.Min = output["min"]
		state.Max = output["max"]
		state.Head = int(output["head"])
		state.Count = int(output["count"])
		state.Ready = output["ready"] != 0
	}

	value := ObserveRank(&state, sample)

	ready := 0.0

	if state.Ready {
		ready = 1
	}

	rank.artifact.Poke(state.History, "history")
	rank.artifact.Poke(datura.Map[float64]{
		"prev":  state.Prev,
		"min":   state.Min,
		"max":   state.Max,
		"head":  float64(state.Head),
		"count": float64(state.Count),
		"ready": ready,
		"value": value,
	}, "output")

	return rank.artifact.Read(p)
}

func (rank *Rank) Close() error {
	return nil
}

/*
ObserveSamples runs the exact batch kernel over samples into out.
*/
func (rank *Rank) ObserveSamples(samples []float64, out []float64) {
	output := datura.Peek[datura.Map[float64]](rank.artifact, "output")
	state := RankState{
		History: datura.Peek[[]float64](rank.artifact, "history"),
	}

	if output != nil {
		state.Prev = output["prev"]
		state.Min = output["min"]
		state.Max = output["max"]
		state.Head = int(output["head"])
		state.Count = int(output["count"])
		state.Ready = output["ready"] != 0
	}

	observeRankSamples(&state, samples, out)

	ready := 0.0

	if state.Ready {
		ready = 1
	}

	lastValue := 0.0

	if len(out) > 0 {
		lastValue = out[len(out)-1]
	}

	rank.artifact.Poke(state.History, "history")
	rank.artifact.Poke(datura.Map[float64]{
		"prev":  state.Prev,
		"min":   state.Min,
		"max":   state.Max,
		"head":  float64(state.Head),
		"count": float64(state.Count),
		"ready": ready,
		"value": lastValue,
	}, "output")
}

/*
Reset clears derived state.
*/
func (rank *Rank) Reset() error {
	rank.artifact.Clear("output")
	rank.artifact.Clear("history")

	return nil
}

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
		rank.artifact.Poke(0.0, "output", "value")
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

	history := datura.Peek[[]float64](rank.artifact, "history")
	minimum := datura.Peek[float64](rank.artifact, "output", "min")
	maximum := datura.Peek[float64](rank.artifact, "output", "max")
	head := int(datura.Peek[float64](rank.artifact, "output", "head"))
	count := int(datura.Peek[float64](rank.artifact, "output", "count"))
	value := 1.0
	extendHistory := count > 1

	if count == 0 {
		minimum = sample
		maximum = sample
		history = make([]float64, 1)
		history[0] = sample
		head = 0
		count = 1
		value = 1
	}

	if count == 1 && sample != minimum {
		extendHistory = true
	}

	if count > 1 {
		minimum = math.Min(minimum, sample)
		maximum = math.Max(maximum, sample)
	}

	if count == 1 && sample != minimum {
		minimum = math.Min(minimum, sample)
		maximum = math.Max(maximum, sample)
	}

	if extendHistory {
		span := maximum - minimum
		capacity := max(1, int(span)+1)

		if len(history) < capacity {
			next := make([]float64, capacity)

			for index := 0; index < count; index++ {
				source := (head - index + len(history)) % len(history)
				next[index] = history[source]
			}

			head = count - 1
			history = next
		}

		head = (head + 1) % len(history)
		history[head] = sample

		if count < len(history) {
			count++
		}

		atOrBelow := 0

		for index := 0; index < count; index++ {
			historyIndex := (head - index + len(history)) % len(history)

			if history[historyIndex] <= sample {
				atOrBelow++
			}
		}

		value = float64(atOrBelow) / float64(count)
	}

	rank.artifact.Poke(history, "history")
	rank.artifact.Poke(sample, "output", "prev")
	rank.artifact.Poke(minimum, "output", "min")
	rank.artifact.Poke(maximum, "output", "max")
	rank.artifact.Poke(float64(head), "output", "head")
	rank.artifact.Poke(float64(count), "output", "count")
	rank.artifact.Poke(value, "output", "value")
	state.MergeOutput("value", value)
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

package probability

import (
	"github.com/theapemachine/nomagique/core"
)

/*
EmpiricalRank tracks P(history <= current sample) over a span-derived window.
*/
type EmpiricalRank[T ~float64] struct {
	state  RankState
	output core.Scalar[T]
}

/*
Rank returns an empirical rank probability dynamic ready from its first observation.
*/
func Rank[T ~float64]() *EmpiricalRank[T] {
	return &EmpiricalRank[T]{}
}

/*
Observe derives the empirical rank probability for the current sample.
*/
func (empiricalRank *EmpiricalRank[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if len(inputs) == 0 {
		return empiricalRank.output
	}

	sample, ok := inputs[0].(core.Scalar[T])

	if !ok {
		return empiricalRank.output
	}

	if len(inputs) > 1 {
		if work, workOK := inputs[1].(core.Scalar[T]); workOK {
			sample = core.Scalar[T](T(sample) + T(work))
		}
	}

	empiricalRank.output = core.Scalar[T](T(
		ObserveRank(&empiricalRank.state, float64(sample)),
	))

	return empiricalRank.output
}

/*
ObserveSamples runs the exact batch kernel over samples into out.
*/
func (empiricalRank *EmpiricalRank[T]) ObserveSamples(samples []float64, out []float64) {
	empiricalRank.state.ObserveSamples(samples, out)
}

/*
Reset clears derived state.
*/
func (empiricalRank *EmpiricalRank[T]) Reset() error {
	empiricalRank.state.Reset()
	empiricalRank.output = core.Scalar[T](0)

	return nil
}

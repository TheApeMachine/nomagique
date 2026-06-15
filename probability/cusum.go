package probability

import (
	"github.com/theapemachine/nomagique/core"
)

/*
ChangeSum accumulates sequential change evidence from a sample stream.
*/
type ChangeSum[T ~float64] struct {
	state  CUSUMState
	output core.Scalar[T]
}

/*
CUSUM returns a change-detection dynamic ready from its first observation.
*/
func CUSUM[T ~float64]() *ChangeSum[T] {
	return &ChangeSum[T]{}
}

/*
Observe derives cumulative change evidence for the current sample.
*/
func (changeSum *ChangeSum[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if len(inputs) == 0 {
		return changeSum.output
	}

	sample, ok := inputs[0].(core.Scalar[T])

	if !ok {
		return changeSum.output
	}

	if len(inputs) > 1 {
		if work, workOK := inputs[1].(core.Scalar[T]); workOK {
			sample = core.Scalar[T](T(sample) + T(work))
		}
	}

	changeSum.output = core.Scalar[T](T(
		ObserveCUSUM(&changeSum.state, float64(sample)),
	))

	return changeSum.output
}

/*
ObserveSamples runs the exact batch kernel over samples into out.
*/
func (changeSum *ChangeSum[T]) ObserveSamples(samples []float64, out []float64) {
	changeSum.state.ObserveSamples(samples, out)
}

/*
Reset clears derived state.
*/
func (changeSum *ChangeSum[T]) Reset() error {
	changeSum.state.Reset()
	changeSum.output = core.Scalar[T](0)

	return nil
}

package probability

import (
	"github.com/theapemachine/nomagique/core"
)

/*
Transition scores transition surprisal from an observed distribution and optionally
records the next category index.
*/
type Transition[T ~float64] struct {
	matrix *TransitionMatrix
	output core.Scalar[T]
}

/*
TransitionSurprise returns a transition surprisal stage for nomagique.Number pipelines.
*/
func TransitionSurprise[T ~float64](numStates int, alpha float64) *Transition[T] {
	return &Transition[T]{
		matrix: NewTransitionMatrix(numStates, alpha),
	}
}

/*
Observe ingests numStates distribution scalars and returns KL surprisal against the
current transition row. When numStates+1 scalars are supplied, the final scalar
records the next category via Update.
*/
func (transition *Transition[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if transition == nil || transition.matrix == nil {
		return core.Scalar[T](0)
	}

	scalars, ok := collectScalars[T](inputs...)

	if !ok || len(scalars) < transition.matrix.numStates {
		return transition.output
	}

	observed := scalars[:transition.matrix.numStates]

	surprise, err := transition.matrix.Surprise(observed)

	if err != nil {
		return transition.output
	}

	if len(scalars) > transition.matrix.numStates {
		transition.matrix.Update(int(scalars[transition.matrix.numStates]))
	}

	transition.output = core.Scalar[T](T(surprise))

	return transition.output
}

/*
Reset clears transition counts back to the smoothing prior.
*/
func (transition *Transition[T]) Reset() error {
	if transition == nil || transition.matrix == nil {
		return nil
	}

	transition.matrix.Reset()
	transition.output = core.Scalar[T](0)

	return nil
}

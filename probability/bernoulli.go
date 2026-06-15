package probability

import (
	"github.com/theapemachine/nomagique/core"
)

/*
Posterior tracks a Beta posterior mean from Bernoulli outcomes.
*/
type Posterior[T ~float64] struct {
	state  BetaState
	output core.Scalar[T]
}

/*
Bernoulli returns a Beta-Bernoulli dynamic ready from its first observation.
*/
func Bernoulli[T ~float64]() *Posterior[T] {
	return &Posterior[T]{}
}

/*
Observe ingests either a unit-interval outcome or a predicted and actual pair.
*/
func (posterior *Posterior[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if len(inputs) == 0 {
		return posterior.output
	}

	scalars, ok := collectScalars[T](inputs...)

	if !ok {
		return posterior.output
	}

	if len(scalars) >= 2 {
		predicted, actual, err := parsePredictedActual(scalars[0], scalars[1:])

		if err != nil {
			return posterior.output
		}

		posterior.output = core.Scalar[T](T(
			ObserveBetaPair(&posterior.state, predicted, actual),
		))

		return posterior.output
	}

	outcome, err := parseBernoulliOutcome(scalars[0], nil)

	if err != nil {
		return posterior.output
	}

	posterior.output = core.Scalar[T](T(ObserveBeta(&posterior.state, outcome)))

	return posterior.output
}

/*
ObserveSamples runs the exact batch kernel over outcomes into out.
*/
func (posterior *Posterior[T]) ObserveSamples(outcomes []float64, out []float64) {
	posterior.state.ObserveSamples(outcomes, out)
}

/*
ObservePairSamples runs the exact batch kernel over pairs into out.
*/
func (posterior *Posterior[T]) ObservePairSamples(
	predicted []float64, actual []float64, out []float64,
) {
	posterior.state.ObservePairSamples(predicted, actual, out)
}

/*
Reset clears derived state.
*/
func (posterior *Posterior[T]) Reset() error {
	posterior.state.Reset()
	posterior.output = core.Scalar[T](0)

	return nil
}

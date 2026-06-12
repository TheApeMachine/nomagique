package probability

import (
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/kernel/prob"
)

/*
Posterior tracks a Beta posterior mean from Bernoulli outcomes.
*/
type Posterior struct {
	stageParser *core.StageParser
	state       prob.BetaState
}

/*
Bernoulli returns a Beta-Bernoulli dynamic ready from its first observation.
*/
func Bernoulli() *Posterior {
	return &Posterior{
		stageParser: core.NewStageParser(),
	}
}

/*
Observe ingests either a unit-interval outcome or a predicted and actual pair.
*/
func (posterior *Posterior) Observe(
	inputs ...core.Number,
) core.Float64 {
	out, work, err := posterior.stageParser.Parse(inputs)

	if err != nil {
		return 0
	}

	result, err := posterior.Apply(out, work)

	if err != nil {
		return 0
	}

	return result
}

/*
Apply runs one pipeline stage without allocating number inputs.
*/
func (posterior *Posterior) Apply(
	out core.Float64, work []core.Float64,
) (core.Float64, error) {
	if len(work) >= 1 {
		predicted, actual, err := parsePredictedActual(out, work)

		if err != nil {
			return 0, err
		}

		return core.Float64(
			prob.ObserveBetaPair(&posterior.state, predicted, actual),
		), nil
	}

	outcome, err := parseBernoulliOutcome(out, work)

	if err != nil {
		return 0, err
	}

	return core.Float64(prob.ObserveBeta(&posterior.state, outcome)), nil
}

/*
ObserveSamples runs the exact batch kernel over outcomes into out.
*/
func (posterior *Posterior) ObserveSamples(outcomes []float64, out []float64) {
	posterior.state.ObserveSamples(outcomes, out)
}

/*
ObservePairSamples runs the exact batch kernel over pairs into out.
*/
func (posterior *Posterior) ObservePairSamples(
	predicted []float64, actual []float64, out []float64,
) {
	posterior.state.ObservePairSamples(predicted, actual, out)
}

/*
Reset clears derived state.
*/
func (posterior *Posterior) Reset() error {
	posterior.state.Reset()
	return nil
}

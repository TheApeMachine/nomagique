package probability

import (
	"math"

	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat/distuv"
)

/*
BernoulliPair carries predicted and actual values for pair-derived outcomes.
*/
type BernoulliPair struct {
	Predicted float64
	Actual    float64
}

/*
BernoulliOutput reports the posterior mean and retained beta state.
*/
type BernoulliOutput struct {
	Value float64
	Alpha float64
	Beta  float64
	Count int
}

/*
Bernoulli tracks a Beta posterior mean from Bernoulli outcomes.
*/
type Bernoulli struct {
	alpha float64
	beta  float64
	prev  float64
	min   float64
	max   float64
	count int
}

/*
NewBernoulli returns a typed Beta-Bernoulli posterior tracker.
*/
func NewBernoulli() *Bernoulli {
	return &Bernoulli{
		alpha: 1,
		beta:  1,
	}
}

/*
Measure adds one Bernoulli outcome in [0, 1].
*/
func (bernoulli *Bernoulli) Measure(outcome float64) (BernoulliOutput, error) {
	if err := finiteProbability("bernoulli", outcome); err != nil {
		return BernoulliOutput{}, err
	}

	parsed, err := parseBernoulliOutcome(outcome, nil)
	if err != nil {
		return BernoulliOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"bernoulli: invalid outcome",
			err,
		))
	}

	return bernoulli.observe(parsed), nil
}

/*
MeasurePair adds one outcome by comparing actual against predicted.
*/
func (bernoulli *Bernoulli) MeasurePair(pair BernoulliPair) (BernoulliOutput, error) {
	if err := finiteProbability("bernoulli", pair.Predicted); err != nil {
		return BernoulliOutput{}, err
	}

	if err := finiteProbability("bernoulli", pair.Actual); err != nil {
		return BernoulliOutput{}, err
	}

	predicted, actual, err := parsePredictedActual(pair.Predicted, []float64{pair.Actual})
	if err != nil {
		return BernoulliOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"bernoulli: unable to parse predicted and actual pair",
			err,
		))
	}

	success := 0.0

	if actual >= predicted {
		success = 1
	}

	tracking := actual - predicted

	if bernoulli.count == 0 {
		bernoulli.prev = predicted
		bernoulli.min = tracking
		bernoulli.max = tracking
	} else {
		bernoulli.min = math.Min(bernoulli.min, tracking)
		bernoulli.max = math.Max(bernoulli.max, tracking)
	}

	bernoulli.prev = predicted

	return bernoulli.observe(success), nil
}

/*
Reset clears posterior state back to its beta(1,1) prior.
*/
func (bernoulli *Bernoulli) Reset() {
	bernoulli.alpha = 1
	bernoulli.beta = 1
	bernoulli.prev = 0
	bernoulli.min = 0
	bernoulli.max = 0
	bernoulli.count = 0
}

func (bernoulli *Bernoulli) observe(outcome float64) BernoulliOutput {
	if bernoulli.count == 0 {
		bernoulli.prev = outcome
		bernoulli.min = outcome
		bernoulli.max = outcome
	} else {
		bernoulli.min = math.Min(bernoulli.min, outcome)
		bernoulli.max = math.Max(bernoulli.max, outcome)
	}

	bernoulli.prev = outcome
	bernoulli.count++
	bernoulli.alpha += outcome
	bernoulli.beta += 1 - outcome
	distribution := distuv.Beta{
		Alpha: bernoulli.alpha,
		Beta:  bernoulli.beta,
	}

	return BernoulliOutput{
		Value: distribution.Mean(),
		Alpha: bernoulli.alpha,
		Beta:  bernoulli.beta,
		Count: bernoulli.count,
	}
}

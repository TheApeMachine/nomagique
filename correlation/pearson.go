package correlation

import (
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/core"
	"gonum.org/v1/gonum/stat"
)

/*
Pearson computes the Pearson correlation coefficient between two streams.
Optionally, weights can be provided which are applied to the inputs before
computing the correlation. This helps to reduce the impact of outliers.
*/
type Pearson struct {
	weights core.Numbers
}

/*
NewPearson creates a new Pearson correlation dynamic.
*/
func NewPearson(weights core.Numbers) *Pearson {
	return &Pearson{
		weights: weights,
	}
}

/*
Observe computes the Pearson correlation coefficient between two streams.
*/
func (pearson *Pearson) Observe(inputs ...core.Number) core.Float64 {
	count := len(inputs)

	if count < 2 {
		errnie.Err(
			errnie.Validation, "unable to compute Pearson correlation",
			PearsonError(PearsonErrorRequireAtLeastTwoInputs),
		)

		return 0
	}

	if count%2 != 0 {
		errnie.Err(
			errnie.Validation, "unable to compute Pearson correlation",
			PearsonError(PearsonErrorRequireEqualLength),
		)

		return 0
	}

	half := count / 2

	left := core.Numbers(inputs[:half])
	right := core.Numbers(inputs[half:])
	weights := nomagique.Samples(pearson.weights)

	if len(weights) == 0 {
		weights = nil
	}

	return core.Float64(stat.Correlation(
		nomagique.Samples(left), nomagique.Samples(right), weights,
	))
}

/*
Reset clears derived state.
*/
func (pearson *Pearson) Reset() error {
	pearson.weights = nil
	return nil
}

type PearsonErrorType string

const (
	PearsonErrorRequireAtLeastTwoInputs PearsonErrorType = "require at least two inputs"
	PearsonErrorRequireEqualLength      PearsonErrorType = "require equal length"
)

type PearsonError string

func (error PearsonError) Error() string {
	return string(error)
}

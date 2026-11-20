package learning

import (
	"errors"
)

/*
ErrEmptyInputs and ErrZeroPredicted let forecast callers distinguish missing
pairs from a predicted value that would blow up log-ratio losses.
*/
var (
	ErrEmptyInputs   = errors.New("learning: empty inputs")
	ErrZeroPredicted = errors.New("learning: zero predicted value")
)

/*
parsePredictedActual accepts both the LearningPair shape and legacy
primary-plus-extras slices so wire formats can evolve without duplicating
validation.
*/
func parsePredictedActual(
	primary float64, extras []float64,
) (float64, float64, error) {
	if len(extras) >= 2 {
		predicted := extras[0]
		actual := extras[1]

		if predicted == 0 {
			return 0, 0, ErrZeroPredicted
		}

		return predicted, actual, nil
	}

	if len(extras) == 0 {
		return 0, 0, ErrEmptyInputs
	}

	predicted := primary
	actual := extras[0]

	if predicted == 0 {
		return 0, 0, ErrZeroPredicted
	}

	return predicted, actual, nil
}

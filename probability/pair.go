package probability

import (
	"github.com/theapemachine/nomagique/core"
)

var (
	ErrEmptyInputs    = core.ErrEmptyInputs
	ErrZeroPredicted  = core.ErrZeroPredicted
	ErrInvalidOutcome = core.ErrInvalidOutcome
)

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

func parseBernoulliOutcome(primary float64, extras []float64) (float64, error) {
	if len(extras) > 0 {
		predicted, actual, err := parsePredictedActual(primary, extras)

		if err != nil {
			return 0, err
		}

		if actual >= predicted {
			return 1, nil
		}

		return 0, nil
	}

	outcome := primary

	if outcome < 0 || outcome > 1 {
		return 0, ErrInvalidOutcome
	}

	return outcome, nil
}

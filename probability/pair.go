package probability

import (
	"github.com/theapemachine/nomagique/core"
)

func parsePredictedActual(
	out core.Float64, work []core.Float64,
) (float64, float64, error) {
	if len(work) == 0 {
		return 0, 0, core.ErrEmptyInputs
	}

	if len(work) >= 2 {
		predicted := float64(work[0])
		actual := float64(work[1])

		if predicted == 0 {
			return 0, 0, core.ErrZeroPredicted
		}

		return predicted, actual, nil
	}

	predicted := float64(out)
	actual := float64(work[0])

	if predicted == 0 {
		return 0, 0, core.ErrZeroPredicted
	}

	return predicted, actual, nil
}

func parseBernoulliOutcome(out core.Float64, work []core.Float64) (float64, error) {
	if len(work) > 0 {
		predicted, actual, err := parsePredictedActual(out, work)

		if err != nil {
			return 0, err
		}

		if actual >= predicted {
			return 1, nil
		}

		return 0, nil
	}

	outcome := float64(out)

	if outcome < 0 || outcome > 1 {
		return 0, core.ErrInvalidOutcome
	}

	return outcome, nil
}

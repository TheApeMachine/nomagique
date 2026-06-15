package probability

import (
	"github.com/theapemachine/nomagique/core"
)

func collectScalars[T ~float64](inputs ...core.Number[T]) ([]float64, bool) {
	scalars := make([]float64, 0, len(inputs))

	for _, input := range inputs {
		scalar, ok := input.(core.Scalar[T])

		if !ok {
			return nil, false
		}

		scalars = append(scalars, float64(scalar))
	}

	return scalars, true
}

func parsePredictedActual(
	primary float64, extras []float64,
) (float64, float64, error) {
	if len(extras) >= 2 {
		predicted := extras[0]
		actual := extras[1]

		if predicted == 0 {
			return 0, 0, core.ErrZeroPredicted
		}

		return predicted, actual, nil
	}

	if len(extras) == 0 {
		return 0, 0, core.ErrEmptyInputs
	}

	predicted := primary
	actual := extras[0]

	if predicted == 0 {
		return 0, 0, core.ErrZeroPredicted
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
		return 0, core.ErrInvalidOutcome
	}

	return outcome, nil
}

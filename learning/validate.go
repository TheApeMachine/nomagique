package learning

import "github.com/theapemachine/errnie"

/*
validatePair is the shared finite, non-zero gate for any path that turns a
LearningPair into a loss or feedback signal.
*/
func validatePair(pair LearningPair, name string) (float64, float64, error) {
	if !finite(pair.Predicted) || !finite(pair.Actual) {
		return 0, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			name+": pair values must be finite",
			nil,
		))
	}

	if pair.Predicted == 0 {
		return 0, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			name+": predicted must be non-zero",
			ErrZeroPredicted,
		))
	}

	if pair.Actual == 0 {
		return 0, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			name+": actual must be non-zero",
			nil,
		))
	}

	return pair.Predicted, pair.Actual, nil
}

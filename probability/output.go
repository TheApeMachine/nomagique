package probability

import (
	"math"

	"github.com/theapemachine/errnie"
)

/*
finiteProbability rejects non-finite inputs before logits or posteriors
propagate NaN into normalized probability vectors.
*/
func finiteProbability(name string, value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return errnie.Error(errnie.Err(
			errnie.Validation,
			name+": value must be finite",
			nil,
		))
	}

	return nil
}

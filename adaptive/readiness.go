package adaptive

import (
	"math"

	"github.com/theapemachine/errnie"
)

/*
finiteAdaptive rejects NaN and Inf before adaptive transforms propagate
invalid values into baselines or ring buffers.
*/
func finiteAdaptive(name string, value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return errnie.Error(errnie.Err(
			errnie.Validation,
			name+": value must be finite",
			nil,
		))
	}

	return nil
}

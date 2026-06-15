package geometry

import (
	"github.com/theapemachine/nomagique/core"
)

func parseGrowthPair(
	primary float64, extras []float64,
) (float64, float64, error) {
	if len(extras) >= 2 {
		return extras[0], extras[1], nil
	}

	if len(extras) == 0 {
		return 0, 0, core.ErrEmptyInputs
	}

	return primary, extras[0], nil
}

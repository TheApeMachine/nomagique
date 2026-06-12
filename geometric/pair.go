package geometric

import (
	"github.com/theapemachine/nomagique/core"
)

func parseGrowthPair(
	out core.Float64, work []core.Float64,
) (float64, float64, error) {
	if len(work) == 0 {
		return 0, 0, core.ErrEmptyInputs
	}

	if len(work) >= 2 {
		return float64(work[0]), float64(work[1]), nil
	}

	return float64(out), float64(work[0]), nil
}

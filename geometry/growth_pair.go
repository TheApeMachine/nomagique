package geometry

import "errors"

var ErrEmptyInputs = errors.New("geometry: empty inputs")

func parseGrowthPair(primary float64, extras []float64) (float64, float64, error) {
	if len(extras) >= 2 {
		return extras[0], extras[1], nil
	}

	if len(extras) == 0 {
		return 0, 0, ErrEmptyInputs
	}

	return primary, extras[0], nil
}

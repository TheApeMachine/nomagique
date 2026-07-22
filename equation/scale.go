package equation

import "math"

/*
inverseSquash maps positive evidence against a supplied positive scale.
*/
func inverseSquash(value float64, scale float64) float64 {
	if value < 0 {
		return 0
	}

	if value == 0 {
		return 1
	}

	if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		return 0
	}

	return scale / (scale + value)
}

/*
magnitudeScale returns the represented binary magnitude of a positive value.
It remains available to equations whose contracts explicitly permit a
value-local scale; ignition does not use it as a baseline substitute.
*/
func magnitudeScale(value float64) float64 {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}

	normalized, exponent := math.Frexp(value)

	if normalized == 0 {
		return 0
	}

	return math.Ldexp(1, exponent-1)
}

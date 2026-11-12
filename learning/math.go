package learning

import "math"

/*
absExact is math.Abs without the NaN edge case — ratio and clip code needs
a sign flip that never turns NaN into a bogus magnitude.
*/
func absExact(value float64) float64 {
	if value < 0 {
		return -value
	}

	return value
}

/*
finite gates every learning update so a poisoned metric cannot propagate into
RLS or manifold state.
*/
func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

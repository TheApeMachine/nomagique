package causal

import "math"

/*
RegimeTracker applies temporal hysteresis so noisy trips do not flip roles.
*/
type RegimeTracker struct {
	inverted      bool
	pendingPanic  int
	pendingNormal int
}

/*
NewRegimeTracker creates a regime tracker.
*/
func NewRegimeTracker() *RegimeTracker {
	return &RegimeTracker{}
}

/*
DeriveRegimeHysteresisSamples returns consecutive samples required before switching.
*/
func DeriveRegimeHysteresisSamples(historyLen int) int {
	if historyLen <= 0 {
		return 2
	}

	samples := int(math.Ceil(math.Sqrt(float64(historyLen))))

	if samples < 2 {
		return 2
	}

	return samples
}

/*
Apply records a raw inverted signal and returns the hysteresis-smoothed state.
*/
func (tracker *RegimeTracker) Apply(rawInverted bool, hysteresis int) bool {
	if hysteresis <= 0 {
		hysteresis = 1
	}

	if rawInverted {
		tracker.pendingPanic++
		tracker.pendingNormal = 0

		if tracker.pendingPanic >= hysteresis {
			tracker.inverted = true
		}
	} else {
		tracker.pendingNormal++
		tracker.pendingPanic = 0

		if tracker.pendingNormal >= hysteresis {
			tracker.inverted = false
		}
	}

	return tracker.inverted
}

/*
Inverted reports the current hysteresis-smoothed inverted state.
*/
func (tracker *RegimeTracker) Inverted() bool {
	return tracker.inverted
}

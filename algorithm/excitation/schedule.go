package excitation

import "math"

/*
schedule invalidates a parameter epoch after newly observed events reach the
sampling-error scale ceil(sqrt(N)) of that fit. Event-count scheduling adapts
to each symbol's activity instead of assigning one wall-clock cooldown to every
market.
*/
type schedule struct {
	threshold int
	changed   int
}

/*
Reset sets the sqrt(N) event threshold after a successful fit publishes.
*/
func (schedule *schedule) Reset(fittedEvents int) {
	schedule.threshold = int(math.Ceil(math.Sqrt(float64(fittedEvents))))
	schedule.changed = 0
}

/*
Observe accumulates genuinely new arrivals toward the next refit epoch.
*/
func (schedule *schedule) Observe(changedEvents int) {
	if changedEvents > 0 {
		schedule.changed += changedEvents
	}
}

/*
Ready reports that enough new events arrived to warrant another Hawkes fit.
*/
func (schedule *schedule) Ready() bool {
	return schedule.threshold > 0 && schedule.changed >= schedule.threshold
}

/*
Remaining counts events still needed before refit; surfaced in projected outcomes.
*/
func (schedule *schedule) Remaining() int {
	remaining := schedule.threshold - schedule.changed

	if remaining > 0 {
		return remaining
	}

	return 0
}

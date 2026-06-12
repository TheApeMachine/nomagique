package hawkes

import (
	"sort"
	"time"
)

/*
Timeline holds sorted event timestamps.
*/
type Timeline struct {
	times []time.Time
}

/*
NewTimeline copies and sorts timestamps.
*/
func NewTimeline(times []time.Time) Timeline {
	sorted := append([]time.Time(nil), times...)
	sort.Slice(sorted, func(leftIndex, rightIndex int) bool {
		return sorted[leftIndex].Before(sorted[rightIndex])
	})

	return Timeline{times: sorted}
}

/*
Times returns the sorted timestamps.
*/
func (timeline Timeline) Times() []time.Time {
	return timeline.times
}

/*
Len returns the event count.
*/
func (timeline Timeline) Len() int {
	return len(timeline.times)
}

/*
Gaps returns inter-arrival gaps in seconds.
*/
func (timeline Timeline) Gaps() []float64 {
	if len(timeline.times) < 2 {
		return nil
	}

	gaps := make([]float64, len(timeline.times)-1)

	for index := 1; index < len(timeline.times); index++ {
		gaps[index-1] = timeline.times[index].Sub(timeline.times[index-1]).Seconds()
	}

	return gaps
}

/*
Span returns seconds from the first event to horizon.
*/
func (timeline Timeline) Span(horizon time.Time) float64 {
	if len(timeline.times) == 0 {
		return 0
	}

	start := timeline.times[0]

	if horizon.Before(start) {
		return 0
	}

	return horizon.Sub(start).Seconds()
}

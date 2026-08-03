package hawkes

import (
	"slices"
	"time"

	"github.com/theapemachine/nomagique/timeline"
)

/*
ArrivalWorkspace builds ephemeral streams while reusing merged-event storage.
A returned stream remains valid until the next call to Stream.
*/
type ArrivalWorkspace struct {
	marked []MarkedEvent
	gaps   gapSummary
}

/*
NewArrivalWorkspace returns an empty reusable stream workspace.
*/
func NewArrivalWorkspace() *ArrivalWorkspace {
	return &ArrivalWorkspace{}
}

/*
Stream builds one sorted arrival view over the caller-provided timelines.
*/
func (workspace *ArrivalWorkspace) Stream(
	buyTimes, sellTimes []time.Time,
) ArrivalStream {
	stream := workspace.StreamFrom(time.Time{}, buyTimes, sellTimes)

	if len(stream.marked) > 0 {
		stream.origin = stream.marked[0].At
	}

	return stream
}

/*
StreamFrom builds one sorted arrival view with an explicit observation origin.
*/
func (workspace *ArrivalWorkspace) StreamFrom(
	origin time.Time,
	buyTimes, sellTimes []time.Time,
) ArrivalStream {
	required := len(buyTimes) + len(sellTimes)
	workspace.marked = slices.Grow(workspace.marked[:0], required)
	workspace.gaps.values = slices.Grow(workspace.gaps.values[:0], required)
	workspace.gaps.sorted = slices.Grow(workspace.gaps.sorted[:0], required)
	stream := ArrivalStream{
		origin: origin,
		buy:    timeline.New(buyTimes),
		sell:   timeline.New(sellTimes),
	}
	workspace.marked = stream.mergeInto(workspace.marked)
	workspace.gaps.reset(workspace.marked)
	stream.marked = workspace.marked
	stream.gaps = workspace.gaps

	return stream
}

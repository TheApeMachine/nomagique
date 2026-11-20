package temporal

import (
	"container/heap"
	"sync"
)

/*
ScheduledEvent is one callback bound to a simulation timestamp and optional
repeat interval so ProcessUntil can reschedule recurring ticks.
*/
type ScheduledEvent struct {
	Time      int64
	Callback  func()
	Repeating bool
	Interval  int64
	id        uint64
}

/*
EventScheduler orders pending callbacks by time and fires them as SimulatedClock
advances, using a side table for in-flight events so Cancel remains reliable.
*/
type EventScheduler struct {
	clock  *SimulatedClock
	events eventHeap
	nextID uint64
	off    *offHeap
	mu     sync.Mutex
}

/*
NewEventScheduler attaches to a simulated clock and initializes the time heap.
*/
func NewEventScheduler(clock *SimulatedClock) *EventScheduler {
	es := &EventScheduler{
		clock:  clock,
		events: make(eventHeap, 0),
		off:    newOffHeap(),
	}
	heap.Init(&es.events)
	return es
}

/*
Schedule enqueues a one-shot callback at atTime simulation nanoseconds.
*/
func (es *EventScheduler) Schedule(atTime int64, callback func()) uint64 {
	es.mu.Lock()
	defer es.mu.Unlock()

	es.nextID++
	event := &ScheduledEvent{
		Time:     atTime,
		Callback: callback,
		id:       es.nextID,
	}
	heap.Push(&es.events, event)
	return es.nextID
}

/*
ScheduleRepeating arms a periodic callback from the current simulation time.
*/
func (es *EventScheduler) ScheduleRepeating(interval int64, callback func()) uint64 {
	es.mu.Lock()
	defer es.mu.Unlock()

	es.nextID++
	event := &ScheduledEvent{
		Time:      es.clock.NowUnixNano() + interval,
		Callback:  callback,
		Repeating: true,
		Interval:  interval,
		id:        es.nextID,
	}
	heap.Push(&es.events, event)
	return es.nextID
}

/*
Cancel removes a pending event or marks an in-flight one so it is not rescheduled.
*/
func (es *EventScheduler) Cancel(id uint64) {
	es.mu.Lock()
	defer es.mu.Unlock()

	for i, event := range es.events {
		if event.id == id {
			heap.Remove(&es.events, i)
			return
		}
	}

	es.off.cancel(id)
}

/*
ProcessUntil fires callbacks with time <= untilTime, invoking each outside the
lock so nested Schedule calls do not deadlock the scheduler.
*/
func (es *EventScheduler) ProcessUntil(untilTime int64) {
	for {
		es.mu.Lock()
		if len(es.events) == 0 || es.events[0].Time > untilTime {
			es.mu.Unlock()
			return
		}

		event := heap.Pop(&es.events).(*ScheduledEvent)
		es.off.hold(event.id)
		es.mu.Unlock()

		event.Callback()

		es.mu.Lock()
		cancelled := es.off.drop(event.id)

		if !cancelled && event.Repeating && event.Interval > 0 {
			event.Time += event.Interval
			heap.Push(&es.events, event)
		}

		es.mu.Unlock()
	}
}

/*
offHeap tracks events popped for Callback so Cancel still reaches them.
*/
type offHeap struct {
	popped    map[uint64]struct{}
	cancelled map[uint64]struct{}
}

/*
newOffHeap allocates per-scheduler cancel tracking for in-flight event IDs.
*/
func newOffHeap() *offHeap {
	return &offHeap{
		popped:    map[uint64]struct{}{},
		cancelled: map[uint64]struct{}{},
	}
}

/*
hold records an event ID as running between Pop and Callback return.
*/
func (set *offHeap) hold(id uint64) {
	set.popped[id] = struct{}{}
}

/*
cancel marks a running event so drop suppresses reschedule after Callback.
*/
func (set *offHeap) cancel(id uint64) {
	if _, running := set.popped[id]; !running {
		return
	}

	set.cancelled[id] = struct{}{}
}

/*
drop clears tracking for id and reports whether the event was cancelled mid-flight.
*/
func (set *offHeap) drop(id uint64) bool {
	_, cancelled := set.cancelled[id]
	delete(set.popped, id)
	delete(set.cancelled, id)

	return cancelled
}

/*
eventHeap is the priority queue backing EventScheduler ordered by simulation time.
*/
type eventHeap []*ScheduledEvent

/*
Len implements heap.Interface for the pending event count.
*/
func (h eventHeap) Len() int { return len(h) }

/*
Less orders by time, then by schedule sequence so equal-timestamp events
fire in FIFO order — heap sift order is not deterministic across runs.
*/
func (h eventHeap) Less(i, j int) bool {
	if h[i].Time != h[j].Time {
		return h[i].Time < h[j].Time
	}
	return h[i].id < h[j].id
}

/*
Swap implements heap.Interface for heap maintenance.
*/
func (h eventHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

/*
Push implements heap.Interface to append a scheduled event pointer.
*/
func (h *eventHeap) Push(x any) {
	*h = append(*h, x.(*ScheduledEvent))
}

/*
Pop implements heap.Interface to remove the earliest scheduled event.
*/
func (h *eventHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

package hawkes

import (
	"slices"
	"time"
)

/*
ArrivalWindow retains one chronological marked-event history. A positive
capacity is a resource bound and evicts the globally oldest event, regardless
of side, so memory pressure cannot distort buy/sell asymmetry. A zero capacity
leaves retention to an explicit time horizon through RetainFrom.

The stream returned by Stream remains valid until the next window method call.
*/
type ArrivalWindow struct {
	events    []MarkedEvent
	buy       []time.Time
	sell      []time.Time
	capacity  int
	workspace *ArrivalWorkspace
}

/*
NewArrivalWindow returns a reusable two-sided arrival window with one optional
total-event resource capacity.
*/
func NewArrivalWindow(capacity int) *ArrivalWindow {
	if capacity < 0 {
		panic("hawkes: arrival window capacity cannot be negative")
	}

	return &ArrivalWindow{
		events:    make([]MarkedEvent, 0, capacity),
		buy:       make([]time.Time, 0, capacity),
		sell:      make([]time.Time, 0, capacity),
		capacity:  capacity,
		workspace: NewArrivalWorkspace(),
	}
}

/*
AppendBuy records the next buy arrival.
*/
func (window *ArrivalWindow) AppendBuy(arrival time.Time) {
	window.append(MarkedEvent{At: arrival, Side: sideBuy})
}

/*
AppendSell records the next sell arrival.
*/
func (window *ArrivalWindow) AppendSell(arrival time.Time) {
	window.append(MarkedEvent{At: arrival, Side: sideSell})
}

/*
RetainFrom discards observations before the statistically selected memory
horizon while preserving every event at the boundary.
*/
func (window *ArrivalWindow) RetainFrom(start time.Time) {
	first := 0

	for first < len(window.events) && window.events[first].At.Before(start) {
		first++
	}

	if first == 0 {
		return
	}

	copy(window.events, window.events[first:])
	window.events = window.events[:len(window.events)-first]
}

/*
Stream returns the current sorted arrival view without allocating.
*/
func (window *ArrivalWindow) Stream() ArrivalStream {
	window.buy = window.buy[:0]
	window.sell = window.sell[:0]

	for _, event := range window.events {
		if event.Side == sideBuy {
			window.buy = append(window.buy, event.At)

			continue
		}

		window.sell = append(window.sell, event.At)
	}

	return window.workspace.Stream(
		window.buy,
		window.sell,
	)
}

func (window *ArrivalWindow) append(event MarkedEvent) {
	window.events = append(window.events, event)

	if len(window.events) > 1 && event.At.Before(window.events[len(window.events)-2].At) {
		slices.SortStableFunc(window.events, func(left, right MarkedEvent) int {
			return left.At.Compare(right.At)
		})
	}

	if window.capacity == 0 || len(window.events) <= window.capacity {
		return
	}

	copy(window.events, window.events[1:])
	window.events = window.events[:window.capacity]
}

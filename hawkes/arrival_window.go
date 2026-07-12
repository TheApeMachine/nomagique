package hawkes

import (
	"slices"
	"time"
)

/*
ArrivalWindow retains a bounded, reusable buy and sell arrival history.
The stream returned by Stream remains valid until the next append.
*/
type ArrivalWindow struct {
	buy       arrivalSide
	sell      arrivalSide
	workspace *ArrivalWorkspace
}

type arrivalSide struct {
	times    []time.Time
	sorted   []time.Time
	capacity int
}

/*
NewArrivalWindow returns a reusable two-sided arrival window.
*/
func NewArrivalWindow(sideCapacity int) *ArrivalWindow {
	if sideCapacity <= 0 {
		panic("hawkes: arrival window capacity must be positive")
	}

	return &ArrivalWindow{
		buy:       newArrivalSide(sideCapacity),
		sell:      newArrivalSide(sideCapacity),
		workspace: NewArrivalWorkspace(),
	}
}

/*
AppendBuy records the next buy arrival.
*/
func (window *ArrivalWindow) AppendBuy(arrival time.Time) {
	window.buy.append(arrival)
}

/*
AppendSell records the next sell arrival.
*/
func (window *ArrivalWindow) AppendSell(arrival time.Time) {
	window.sell.append(arrival)
}

/*
Stream returns the current sorted arrival view without allocating.
*/
func (window *ArrivalWindow) Stream() ArrivalStream {
	return window.workspace.Stream(
		window.buy.ordered(),
		window.sell.ordered(),
	)
}

func newArrivalSide(capacity int) arrivalSide {
	return arrivalSide{
		times:    make([]time.Time, 0, capacity),
		sorted:   make([]time.Time, 0, capacity),
		capacity: capacity,
	}
}

func (side *arrivalSide) append(arrival time.Time) {
	if len(side.times) < side.capacity {
		side.times = append(side.times, arrival)

		return
	}

	copy(side.times, side.times[1:])
	side.times[len(side.times)-1] = arrival
}

func (side *arrivalSide) ordered() []time.Time {
	for index := 1; index < len(side.times); index++ {
		if !side.times[index].Before(side.times[index-1]) {
			continue
		}

		side.sorted = append(side.sorted[:0], side.times...)
		slices.SortFunc(side.sorted, func(left, right time.Time) int {
			return left.Compare(right)
		})

		return side.sorted
	}

	return side.times
}

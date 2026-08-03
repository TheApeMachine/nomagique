package hawkes

import (
	"sort"
	"time"
)

/*
Between returns arrivals inside [start, end] and sets the counted observation
interval to (start, end]. An event at start remains available as prehistory.
*/
func (stream ArrivalStream) Between(start, end time.Time) ArrivalStream {
	buyTimes := betweenTimes(stream.BuyTimes(), start, end)
	sellTimes := betweenTimes(stream.SellTimes(), start, end)

	if len(buyTimes) == len(stream.BuyTimes()) &&
		len(sellTimes) == len(stream.SellTimes()) {
		return stream.WithObservationOrigin(start)
	}

	return NewArrivalStreamFrom(start, buyTimes, sellTimes)
}

/*
BetweenInto returns an interval stream using caller-owned reusable storage.
*/
func (stream ArrivalStream) BetweenInto(
	start, end time.Time,
	workspace *ArrivalWorkspace,
) ArrivalStream {
	buyTimes := betweenTimes(stream.BuyTimes(), start, end)
	sellTimes := betweenTimes(stream.SellTimes(), start, end)

	if len(buyTimes) == len(stream.BuyTimes()) &&
		len(sellTimes) == len(stream.SellTimes()) {
		return stream.WithObservationOrigin(start)
	}

	return workspace.StreamFrom(start, buyTimes, sellTimes)
}

func betweenTimes(times []time.Time, start, end time.Time) []time.Time {
	first := sort.Search(len(times), func(index int) bool {
		return !times[index].Before(start)
	})
	last := sort.Search(len(times), func(index int) bool {
		return times[index].After(end)
	})

	if first >= last {
		return nil
	}

	return times[first:last]
}

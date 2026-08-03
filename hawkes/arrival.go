package hawkes

import (
	"slices"
	"time"

	"github.com/theapemachine/nomagique/decay"
	"github.com/theapemachine/nomagique/timeline"
)

type eventSide int

const (
	sideBuy eventSide = iota
	sideSell
)

/*
MarkedEvent is one arrival tagged by stream side.
*/
type MarkedEvent struct {
	At   time.Time
	Side eventSide
}

/*
ArrivalStream holds sorted buy and sell timelines inside one measurement window.
*/
type ArrivalStream struct {
	origin time.Time
	buy    timeline.Timeline
	sell   timeline.Timeline
	marked []MarkedEvent
	gaps   gapSummary
}

/*
NewArrivalStream sorts both arrival timelines and merges one arrival reading.
*/
func NewArrivalStream(buyTimes, sellTimes []time.Time) ArrivalStream {
	stream := ArrivalStream{
		buy:  timeline.New(buyTimes),
		sell: timeline.New(sellTimes),
	}
	stream.marked = stream.merge()
	stream.gaps.reset(stream.marked)

	if len(stream.marked) > 0 {
		stream.origin = stream.marked[0].At
	}

	return stream
}

/*
NewArrivalStreamFrom constructs a stream with an explicit observation origin.
Events at or before origin are retained as excitation prehistory but are not
counted observations. Counted arrivals therefore follow (origin, horizon].
*/
func NewArrivalStreamFrom(
	origin time.Time,
	buyTimes, sellTimes []time.Time,
) ArrivalStream {
	stream := NewArrivalStream(buyTimes, sellTimes)
	stream.origin = origin

	return stream
}

/*
ObservationOrigin returns the common left endpoint for both marked sides.
*/
func (stream ArrivalStream) ObservationOrigin() time.Time {
	return stream.origin
}

/*
WithObservationOrigin returns the same arrival support with a new common
observation origin. Events at the origin remain available as prehistory.
*/
func (stream ArrivalStream) WithObservationOrigin(origin time.Time) ArrivalStream {
	stream.origin = origin

	return stream
}

/*
BuyTimes returns buy-side timestamps.
*/
func (stream ArrivalStream) BuyTimes() []time.Time {
	return stream.buy.Times()
}

/*
SellTimes returns sell-side timestamps.
*/
func (stream ArrivalStream) SellTimes() []time.Time {
	return stream.sell.Times()
}

/*
Marked returns a snapshot of buy and sell events in chronological order.
*/
func (stream ArrivalStream) Marked() []MarkedEvent {
	return slices.Clone(stream.marked)
}

func (stream ArrivalStream) markedEvents() []MarkedEvent {
	return stream.marked
}

func (stream ArrivalStream) merge() []MarkedEvent {
	return stream.mergeInto(make([]MarkedEvent, 0, stream.buy.Len()+stream.sell.Len()))
}

func (stream ArrivalStream) mergeInto(marked []MarkedEvent) []MarkedEvent {
	buyTimes := stream.buy.Times()
	sellTimes := stream.sell.Times()
	buyIndex := 0
	sellIndex := 0

	for buyIndex < len(buyTimes) && sellIndex < len(sellTimes) {
		if !buyTimes[buyIndex].After(sellTimes[sellIndex]) {
			marked = append(marked, MarkedEvent{
				At: buyTimes[buyIndex], Side: sideBuy,
			})

			buyIndex++
			continue
		}

		marked = append(marked, MarkedEvent{
			At: sellTimes[sellIndex], Side: sideSell,
		})

		sellIndex++
	}

	for buyIndex < len(buyTimes) {
		marked = append(marked, MarkedEvent{
			At: buyTimes[buyIndex], Side: sideBuy,
		})

		buyIndex++
	}

	for sellIndex < len(sellTimes) {
		marked = append(marked, MarkedEvent{
			At: sellTimes[sellIndex], Side: sideSell,
		})

		sellIndex++
	}

	return marked
}

/*
Gaps returns inter-arrival gaps across marked events.
*/
func (stream ArrivalStream) Gaps() []float64 {
	return slices.Clone(stream.gaps.values)
}

/*
Bounds returns the earliest and latest marked arrival.
*/
func (stream ArrivalStream) Bounds() (time.Time, time.Time, bool) {
	if len(stream.marked) == 0 {
		return time.Time{}, time.Time{}, false
	}

	return stream.marked[0].At, stream.marked[len(stream.marked)-1].At, true
}

/*
Span returns exposure seconds on the common interval (origin, horizon].
*/
func (stream ArrivalStream) Span(horizon time.Time) float64 {
	if stream.origin.IsZero() || !horizon.After(stream.origin) {
		return 0
	}

	return horizon.Sub(stream.origin).Seconds()
}

func (stream ArrivalStream) observationCounts(horizon time.Time) (buy, sell int) {
	return observationCount(stream.buy.Times(), stream.origin, horizon),
		observationCount(stream.sell.Times(), stream.origin, horizon)
}

/*
ObservationCounts returns side counts on the common interval (origin, horizon].
*/
func (stream ArrivalStream) ObservationCounts(horizon time.Time) (buy, sell int) {
	return stream.observationCounts(horizon)
}

func observationCount(times []time.Time, origin, horizon time.Time) int {
	count := 0

	for _, eventTime := range times {
		if !eventTime.After(origin) {
			continue
		}

		if eventTime.After(horizon) {
			break
		}

		count++
	}

	return count
}

func (stream ArrivalStream) observationMarked(horizon time.Time) []MarkedEvent {
	marked := make([]MarkedEvent, 0, len(stream.marked))

	for _, event := range stream.marked {
		if !event.At.After(stream.origin) {
			continue
		}

		if event.At.After(horizon) {
			break
		}

		marked = append(marked, event)
	}

	return marked
}

func (stream ArrivalStream) buyIntensityAt(
	horizon time.Time,
	muBuy, alphaBB, alphaBS, beta float64,
) float64 {
	return decay.IntensityAt(
		stream.buy,
		stream.sell,
		horizon,
		muBuy,
		alphaBB,
		alphaBS,
		beta,
	)
}

func (stream ArrivalStream) sellIntensityAt(
	horizon time.Time,
	muSell, alphaSB, alphaSS, beta float64,
) float64 {
	return decay.IntensityAt(
		stream.buy,
		stream.sell,
		horizon,
		muSell,
		alphaSB,
		alphaSS,
		beta,
	)
}

func (stream ArrivalStream) kernelIntegralSupport(
	horizon time.Time, beta float64,
) (buy, sell float64) {
	return observationKernelIntegralSupport(
			stream.buy.Times(), stream.origin, horizon, beta,
		), observationKernelIntegralSupport(
			stream.sell.Times(), stream.origin, horizon, beta,
		)
}

func observationKernelIntegralSupport(
	events []time.Time,
	origin, horizon time.Time,
	beta float64,
) float64 {
	support := 0.0

	for _, eventTime := range events {
		if eventTime.After(horizon) {
			break
		}

		lowerAge := origin.Sub(eventTime).Seconds()

		if lowerAge < 0 {
			lowerAge = 0
		}

		upperAge := horizon.Sub(eventTime).Seconds()

		if upperAge <= lowerAge {
			continue
		}

		support += decay.ExpNeg(beta, lowerAge) - decay.ExpNeg(beta, upperAge)
	}

	return support
}

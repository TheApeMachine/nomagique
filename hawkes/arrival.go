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
Span returns seconds from the first marked event to horizon.
*/
func (stream ArrivalStream) Span(horizon time.Time) float64 {
	marked := stream.markedEvents()

	if len(marked) == 0 || horizon.Before(marked[0].At) {
		return 0
	}

	return horizon.Sub(marked[0].At).Seconds()
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
	return decay.KernelIntegralSupport(
			stream.buy, horizon, beta,
		), decay.KernelIntegralSupport(
			stream.sell, horizon, beta,
		)
}

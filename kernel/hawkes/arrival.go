package hawkes

import (
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
	buy  timeline.Timeline
	sell timeline.Timeline
}

/*
NewArrivalStream copies and sorts both arrival timelines.
*/
func NewArrivalStream(buyTimes, sellTimes []time.Time) ArrivalStream {
	return ArrivalStream{
		buy:  timeline.New(buyTimes),
		sell: timeline.New(sellTimes),
	}
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
Marked merges buy and sell events in chronological order.
*/
func (stream ArrivalStream) Marked() []MarkedEvent {
	buyTimes := stream.buy.Times()
	sellTimes := stream.sell.Times()
	marked := make([]MarkedEvent, 0, len(buyTimes)+len(sellTimes))
	buyIndex := 0
	sellIndex := 0

	for buyIndex < len(buyTimes) && sellIndex < len(sellTimes) {
		if !buyTimes[buyIndex].After(sellTimes[sellIndex]) {
			marked = append(marked, MarkedEvent{At: buyTimes[buyIndex], Side: sideBuy})
			buyIndex++
			continue
		}

		marked = append(marked, MarkedEvent{At: sellTimes[sellIndex], Side: sideSell})
		sellIndex++
	}

	for buyIndex < len(buyTimes) {
		marked = append(marked, MarkedEvent{At: buyTimes[buyIndex], Side: sideBuy})
		buyIndex++
	}

	for sellIndex < len(sellTimes) {
		marked = append(marked, MarkedEvent{At: sellTimes[sellIndex], Side: sideSell})
		sellIndex++
	}

	return marked
}

func (stream ArrivalStream) markedTimeline() timeline.Timeline {
	marked := stream.Marked()
	times := make([]time.Time, len(marked))

	for index, event := range marked {
		times[index] = event.At
	}

	return timeline.New(times)
}

/*
Gaps returns inter-arrival gaps across marked events.
*/
func (stream ArrivalStream) Gaps() []float64 {
	return stream.markedTimeline().Gaps()
}

/*
Span returns seconds from the first marked event to horizon.
*/
func (stream ArrivalStream) Span(horizon time.Time) float64 {
	return stream.markedTimeline().Span(horizon)
}

func (stream ArrivalStream) buyIntensityAt(
	horizon time.Time,
	muBuy, alphaBB, alphaBS, beta float64,
) float64 {
	return decay.IntensityAt(stream.buy, stream.sell, horizon, muBuy, alphaBB, alphaBS, beta)
}

func (stream ArrivalStream) sellIntensityAt(
	horizon time.Time,
	muSell, alphaSB, alphaSS, beta float64,
) float64 {
	return decay.IntensityAt(stream.buy, stream.sell, horizon, muSell, alphaSB, alphaSS, beta)
}

func (stream ArrivalStream) kernelSupport(horizon time.Time, beta float64) (buy, sell float64) {
	return decay.KernelSupport(stream.buy, horizon, beta),
		decay.KernelSupport(stream.sell, horizon, beta)
}

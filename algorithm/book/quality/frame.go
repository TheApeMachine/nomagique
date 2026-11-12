package quality

import (
	"math"
	"sort"

	"github.com/theapemachine/nomagique/algorithm/book/flow"
)

/*
Frame accumulates per-frame cancel, fill, and touch-add volume before a window
finish builds equation.BookQualityInput for symm envelope wiring.
*/
type Frame struct {
	cancelBid      float64
	fillBid        float64
	cancelAsk      float64
	fillAsk        float64
	addBid         float64
	addAsk         float64
	touchCancelBid float64
	touchCancelAsk float64
}

/*
FrameAdd records resting liquidity added on one side during a book frame.
*/
func FrameAdd(frame *Frame, side flow.Side, quantity float64) {
	if side == flow.SideBid {
		frame.addBid += quantity

		return
	}

	frame.addAsk += quantity
}

/*
FrameFill records matched volume attributed to one side for cancel/fill ratios.
*/
func FrameFill(frame *Frame, side flow.Side, quantity float64) {
	if side == flow.SideBid {
		frame.fillBid += quantity

		return
	}

	frame.fillAsk += quantity
}

/*
FrameCancel records withdrawn quantity, tagging touch cancels for bluff churn.
*/
func FrameCancel(
	frame *Frame,
	side flow.Side,
	quantity float64,
	touch bool,
) {
	if side == flow.SideBid {
		frame.cancelBid += quantity

		if touch {
			frame.touchCancelBid += quantity
		}

		return
	}

	frame.cancelAsk += quantity

	if touch {
		frame.touchCancelAsk += quantity
	}
}

/*
CancelFillRatio is the vacuum/support ratio equation.BookQuality expects from sampled flow.
*/
func CancelFillRatio(cancel, fill float64) float64 {
	if cancel <= 0 || fill <= 0 {
		return 0
	}

	return cancel / fill
}

/*
TradedAt corroborates L3 deletes as fills when prints land near recent trade prices.
*/
func TradedAt(price float64, trades []float64) bool {
	if price <= 0 || len(trades) == 0 {
		return false
	}

	tolerance := MedianAbsoluteDeviation(trades)

	for _, traded := range trades {
		if math.Abs(traded-price) <= tolerance {
			return true
		}
	}

	return false
}

/*
MedianAbsoluteDeviation sets trade-matching tolerance from the local print distribution.
*/
func MedianAbsoluteDeviation(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}

	center := Median(values)
	deviations := make([]float64, 0, len(values))

	for _, value := range values {
		deviations = append(deviations, math.Abs(value-center))
	}

	return Median(deviations)
}

/*
Median is a robust center for touch tolerance without pulling in heavy statistics deps.
*/
func Median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2

	if len(sorted)%2 == 1 {
		return sorted[mid]
	}

	return (sorted[mid-1] + sorted[mid]) / 2
}

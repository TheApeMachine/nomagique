package algorithm

import (
	"math"
	"sort"
)

type bookQualityFrame struct {
	cancelBid      float64
	fillBid        float64
	cancelAsk      float64
	fillAsk        float64
	addBid         float64
	addAsk         float64
	touchCancelBid float64
	touchCancelAsk float64
}

func bookQualityFrameAdd(frame *bookQualityFrame, side byte, quantity float64) {
	if side == SideBid {
		frame.addBid += quantity

		return
	}

	frame.addAsk += quantity
}

func bookQualityFrameFill(frame *bookQualityFrame, side byte, quantity float64) {
	if side == SideBid {
		frame.fillBid += quantity

		return
	}

	frame.fillAsk += quantity
}

func bookQualityFrameCancel(
	frame *bookQualityFrame,
	side byte,
	quantity float64,
	touch bool,
) {
	if side == SideBid {
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

func bookQualityCancelFillRatio(cancel, fill float64) float64 {
	if cancel <= 0 || fill <= 0 {
		return 0
	}

	return cancel / fill
}

func bookQualityTradedAt(price float64, trades []float64) bool {
	if price <= 0 || len(trades) == 0 {
		return false
	}

	tolerance := bookQualityMedianAbsoluteDeviation(trades)

	for _, traded := range trades {
		if math.Abs(traded-price) <= tolerance {
			return true
		}
	}

	return false
}

func bookQualityMedianAbsoluteDeviation(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}

	center := bookQualityMedian(values)
	deviations := make([]float64, 0, len(values))

	for _, value := range values {
		deviations = append(deviations, math.Abs(value-center))
	}

	return bookQualityMedian(deviations)
}

func bookQualityMedian(values []float64) float64 {
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

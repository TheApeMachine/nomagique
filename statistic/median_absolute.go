package statistic

import (
	"math"

	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/core"
)

/*
MedianAbsolute measures typical magnitude while ignoring sign.

Each input is converted to its absolute value, then Median runs on that batch.
Trading example: median absolute return tells you how much symbols move on average
without caring whether the move was up or down — useful for energy and volatility
rankings.

MedianAbsolute implements core.Number and delegates the actual median to Median.
*/
type MedianAbsolute struct {
	weights core.Numbers
}

/*
NewMedianAbsolute creates a median-absolute dynamic.
*/
func NewMedianAbsolute(weights core.Numbers) *MedianAbsolute {
	return &MedianAbsolute{
		weights: weights,
	}
}

/*
Observe returns the median absolute value of the input stream.
*/
func (medianAbsolute *MedianAbsolute) Observe(inputs ...core.Number) core.Float64 {
	values := nomagique.Samples(core.Numbers(inputs))

	if len(values) == 0 {
		return 0
	}

	absolute := make([]core.Number, len(values))

	for index, value := range values {
		absolute[index] = core.Float64(math.Abs(value))
	}

	return NewMedian(medianAbsolute.weights).Observe(absolute...)
}

func (medianAbsolute *MedianAbsolute) Reset() error {
	medianAbsolute.weights = nil
	return nil
}

package correlation

import (
	"github.com/theapemachine/nomagique/core"
)

/*
IntervalCoupling tracks Hayashi-Yoshida correlation between two interval accumulators.
*/
type IntervalCoupling struct {
	left  *IntervalSeries
	right *IntervalSeries
}

/*
NewIntervalCoupling creates an interval-correlation dynamic over two series.
*/
func NewIntervalCoupling(left, right *IntervalSeries) *IntervalCoupling {
	return &IntervalCoupling{
		left:  left,
		right: right,
	}
}

/*
Observe returns the interval correlation between the configured series.
*/
func (coupling *IntervalCoupling) Observe(_ ...core.Number) core.Float64 {
	if coupling == nil {
		return 0
	}

	value, ok := IntervalCorrelation(coupling.left, coupling.right)

	if !ok {
		return 0
	}

	return core.Float64(value)
}

/*
Reset is a no-op; interval history lives in the series.
*/
func (coupling *IntervalCoupling) Reset() error {
	return nil
}

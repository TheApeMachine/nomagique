package correlation

import (
	"github.com/theapemachine/nomagique/core"
)

/*
IntervalCoupling tracks Hayashi-Yoshida correlation between two interval accumulators.
*/
type IntervalCoupling[T ~float64] struct {
	left   *IntervalSeries[T]
	right  *IntervalSeries[T]
	output core.Scalar[T]
}

/*
NewIntervalCoupling creates an interval-correlation dynamic over two series.
*/
func NewIntervalCoupling[T ~float64](
	left, right *IntervalSeries[T],
) *IntervalCoupling[T] {
	return &IntervalCoupling[T]{
		left:  left,
		right: right,
	}
}

/*
Observe returns the interval correlation between the configured series.
*/
func (coupling *IntervalCoupling[T]) Observe(_ ...core.Number[T]) core.Scalar[T] {
	if coupling == nil {
		return core.Scalar[T](0)
	}

	value, ok := IntervalCorrelation(coupling.left, coupling.right)

	if !ok {
		return coupling.output
	}

	coupling.output = core.Scalar[T](T(value))

	return coupling.output
}

/*
Reset is a no-op; interval history lives in the series.
*/
func (coupling *IntervalCoupling[T]) Reset() error {
	coupling.output = core.Scalar[T](0)

	return nil
}

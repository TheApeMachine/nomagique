package correlation

import (
	"github.com/theapemachine/nomagique/core"
)

/*
TierWindows names fast, medium, and slow interval counts for multi-scale reads.
*/
type TierWindows struct {
	Fast   int
	Medium int
	Slow   int
}

/*
WindowSnapshot holds independent tier views materialized from one live series.
*/
type WindowSnapshot[T ~float64] struct {
	Fast   *IntervalSeries[T]
	Medium *IntervalSeries[T]
	Slow   *IntervalSeries[T]
}

/*
WindowSet holds one live interval series and materializes tier views on demand.
*/
type WindowSet[T ~float64] struct {
	series *IntervalSeries[T]
	output core.Scalar[T]
}

/*
NewWindowSet creates a window set backed by a bounded interval series.
*/
func NewWindowSet[T ~float64](capacity int) *WindowSet[T] {
	return &WindowSet[T]{
		series: NewIntervalSeries[T](capacity),
	}
}

/*
Observe ingests epoch nanoseconds and a positive level as two scalar inputs.
*/
func (windowSet *WindowSet[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if windowSet == nil || windowSet.series == nil {
		return core.Scalar[T](0)
	}

	windowSet.output = windowSet.series.Observe(inputs...)

	return windowSet.output
}

/*
Reset clears the live interval history.
*/
func (windowSet *WindowSet[T]) Reset() error {
	if windowSet == nil || windowSet.series == nil {
		return nil
	}

	windowSet.output = core.Scalar[T](0)

	return windowSet.series.Reset()
}

/*
Series returns the live interval accumulator.
*/
func (windowSet *WindowSet[T]) Series() *IntervalSeries[T] {
	if windowSet == nil {
		return nil
	}

	return windowSet.series
}

/*
Snapshot clones fast, medium, and slow tier views from the current live history.
*/
func (windowSet *WindowSet[T]) Snapshot(tiers TierWindows) WindowSnapshot[T] {
	if windowSet == nil || windowSet.series == nil {
		return WindowSnapshot[T]{}
	}

	return WindowSnapshot[T]{
		Fast:   windowSet.series.CloneTail(tiers.Fast),
		Medium: windowSet.series.CloneTail(tiers.Medium),
		Slow:   windowSet.series.CloneTail(tiers.Slow),
	}
}

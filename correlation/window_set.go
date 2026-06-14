package correlation

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
type WindowSnapshot struct {
	Fast   *IntervalSeries
	Medium *IntervalSeries
	Slow   *IntervalSeries
}

/*
WindowSet holds one live interval series and materializes tier views on demand.

WindowSet.Observe(nanos, price) is a feed adapter, not core.Number. Push trade
prints here, then pass WindowSnapshot into correlation stages such as Multiverse.
*/
type WindowSet struct {
	series *IntervalSeries
}

/*
NewWindowSet creates a window set backed by a bounded interval series.
*/
func NewWindowSet(capacity int) *WindowSet {
	return &WindowSet{
		series: NewIntervalSeries(capacity),
	}
}

/*
Observe folds one trade print into the live series.

This is not core.Number.Observe — it accepts raw timestamp and price directly.
*/
func (windowSet *WindowSet) Observe(nanos int64, price float64) {
	if windowSet == nil || windowSet.series == nil {
		return
	}

	windowSet.series.Observe(nanos, price)
}

/*
Series returns the live interval accumulator.
*/
func (windowSet *WindowSet) Series() *IntervalSeries {
	if windowSet == nil {
		return nil
	}

	return windowSet.series
}

/*
Snapshot clones fast, medium, and slow tier views from the current live history.
*/
func (windowSet *WindowSet) Snapshot(tiers TierWindows) WindowSnapshot {
	if windowSet == nil || windowSet.series == nil {
		return WindowSnapshot{}
	}

	return WindowSnapshot{
		Fast:   windowSet.series.CloneTail(tiers.Fast),
		Medium: windowSet.series.CloneTail(tiers.Medium),
		Slow:   windowSet.series.CloneTail(tiers.Slow),
	}
}

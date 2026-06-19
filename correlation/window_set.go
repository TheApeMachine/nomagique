package correlation

import (
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
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
type WindowSnapshot struct {
	Fast   *IntervalSeries
	Medium *IntervalSeries
	Slow   *IntervalSeries
}

/*
WindowSet holds one live interval series and materializes tier views on demand.
*/
type WindowSet struct {
	artifact *datura.Artifact
	series   *IntervalSeries
}

/*
NewWindowSet creates a window set backed by a bounded interval series.
*/
func NewWindowSet(capacity int) *WindowSet {
	return &WindowSet{
		artifact: datura.Acquire("window-set", datura.APPJSON).RetainStageAttributes(),
		series:   NewIntervalSeries(capacity),
	}
}

func (windowSet *WindowSet) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](windowSet.artifact, "output") == nil

	windowSet.artifact.Clear("sample")
	windowSet.artifact.Clear("paired")

	n, err := windowSet.artifact.Write(p)

	if bootstrap {
		windowSet.artifact.Clear("output")
	}

	return n, err
}

func (windowSet *WindowSet) Read(p []byte) (int, error) {
	if windowSet == nil || windowSet.series == nil {
		return windowSet.artifact.Read(p)
	}

	level := datura.Peek[float64](windowSet.artifact, "paired")

	if level <= 0 {
		return windowSet.artifact.Read(p)
	}

	inbound := datura.Acquire("inbound", datura.APPJSON).
		Poke(datura.Peek[float64](windowSet.artifact, "sample"), "sample").
		Poke(level, "paired")

	err := transport.NewFlipFlop(inbound, windowSet.series)

	if err != nil {
		return windowSet.artifact.Read(p)
	}

	windowSet.artifact.Poke(
		datura.Map[float64]{"value": windowSet.series.LastReturnMagnitude()},
		"output",
	)

	return windowSet.artifact.Read(p)
}

func (windowSet *WindowSet) Close() error {
	return nil
}

func (windowSet *WindowSet) Reset() error {
	if windowSet == nil || windowSet.series == nil {
		return nil
	}

	windowSet.artifact.Clear("output")

	return windowSet.series.Reset()
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

package correlation

import (
	"io"

	"github.com/theapemachine/datura"
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
	output   float64
}

/*
NewWindowSet creates a window set backed by a bounded interval series.
*/
func NewWindowSet(capacity int) *WindowSet {
	return &WindowSet{
		artifact: datura.Acquire("window-set", datura.Artifact_Type_json),
		series:   NewIntervalSeries(capacity),
	}
}

func (windowSet *WindowSet) Write(p []byte) (int, error) {
	return windowSet.artifact.Write(p)
}

func (windowSet *WindowSet) Read(p []byte) (int, error) {
	if windowSet == nil || windowSet.series == nil {
		return windowSet.artifact.Read(p)
	}

	inbound, inboundOK := artifactBytes(windowSet.artifact)

	if !inboundOK {
		return windowSet.artifact.Read(p)
	}

	_, writeErr := windowSet.series.Write(inbound)

	if writeErr != nil {
		return windowSet.artifact.Read(p)
	}

	outBuf := make([]byte, 4096)
	_, readErr := windowSet.series.Read(outBuf)

	if readErr != nil && readErr != io.EOF && readErr != io.ErrShortBuffer {
		return windowSet.artifact.Read(p)
	}

	windowSet.output = seriesOutput(windowSet.series)
	putFloat64Payload(&windowSet.artifact, "window-set", windowSet.output)

	return windowSet.artifact.Read(p)
}

func (windowSet *WindowSet) Close() error {
	return nil
}

func (windowSet *WindowSet) Reset() error {
	if windowSet == nil || windowSet.series == nil {
		return nil
	}

	windowSet.output = 0

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

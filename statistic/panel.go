package statistic

import (
	"sync"

	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/core"
)

/*
Panel is a live registry of keyed numeric samples — think of it as a row in a
spreadsheet where each row is an asset (or any member) and the cell holds that
member's latest reading.

Trading example: one row per symbol, each cell holding that symbol's session
change percent. The panel itself does not compute a summary; it only remembers
the latest value for every member so downstream stages can aggregate across the
universe.

Panel implements core.Number. Feed it through nomagique.Number(...) or call
Observe directly with two inputs: member key, then sample value. Keys are
float64 identifiers (callers map strings or indices to keys). The returned
value is the sample that was just stored.
*/
type Panel struct {
	values sync.Map
}

/*
Observe registers or updates one member's latest sample.

Inputs (in order):
 1. member key — which row in the panel (e.g. a numeric symbol id)
 2. sample value — the reading to store (e.g. change percent, return, volume)

Fewer than two inputs returns zero and stores nothing. This keeps the panel
usable inside a nomagique pipeline without silent partial writes.

Typical usage outside a composed Number:

	panel.Observe(nomagique.Scalar(memberKey), nomagique.Scalar(changePct))
*/
func (panel *Panel) Observe(inputs ...core.Number) core.Float64 {
	samples := nomagique.Samples(core.Numbers(inputs))

	if len(samples) < 2 {
		return 0
	}

	memberKey := samples[0]
	panel.values.Store(memberKey, samples[1])

	return core.Float64(samples[1])
}

/*
Reset clears every stored member sample.

Use when the universe rolls (new session, subscription change) so stale keys do
not leak into later cross-section reads.
*/
func (panel *Panel) Reset() error {
	panel.values = sync.Map{}

	return nil
}

/*
LeaveOneOutMedian answers: "What is the typical peer reading if we ignore one
member?"

Given a Panel of member samples, it collects every value except the excluded
member key and passes those peer values to Median. This is the usual way to
estimate macro or cross-section drift without letting a symbol vote on its own
baseline — e.g. median change of all other symbols while measuring one asset's
local causal story.

Compose with nomagique.Number:

	panel := statistic.Panel{}
	leaveOneOut := statistic.NewLeaveOneOutMedian(&panel)
	macro, _ := nomagique.Number(leaveOneOut)

Register peers on the panel, then query with the excluded key as the boundary
scalar observed through the composed Number.
*/
type LeaveOneOutMedian struct {
	panel  *Panel
	median *Median
}

/*
NewLeaveOneOutMedian binds a Panel to a leave-one-out median stage.

The same Panel pointer must be shared between registration (Panel.Observe) and
aggregation (LeaveOneOutMedian.Observe). One panel can feed many composed
Numbers as long as they all reference the same underlying registry.
*/
func NewLeaveOneOutMedian(panel *Panel) *LeaveOneOutMedian {
	return &LeaveOneOutMedian{
		panel:  panel,
		median: NewMedian(nil),
	}
}

/*
Observe returns the median of all panel members except the excluded key.

Input: the member key to leave out (one boundary scalar when called directly).

When observed through a nomagique.Number pipeline, the excluded key is the
carried boundary sample (the last input after pipeline wiring), not the initial
zero placeholder.

Returns zero when the panel is empty, the excluded key is unknown, or no peers
remain after exclusion. Median math is delegated to Median.Observe — this stage
only selects peers; it does not reimplement order statistics.
*/
func (leaveOneOut *LeaveOneOutMedian) Observe(inputs ...core.Number) core.Float64 {
	samples := nomagique.Samples(core.Numbers(inputs))

	if len(samples) == 0 {
		return 0
	}

	excludedKey := samples[0]

	if len(samples) > 1 {
		excludedKey = samples[len(samples)-1]
	}

	peerSamples := leaveOneOut.peerSamples(excludedKey)

	if len(peerSamples) == 0 {
		return 0
	}

	return leaveOneOut.median.Observe(nomagique.Numbers(peerSamples...)...)
}

/*
Reset clears derived median state on the embedded Median stage.

Panel member samples are not cleared here; call Panel.Reset when the universe
should forget prior registrations.
*/
func (leaveOneOut *LeaveOneOutMedian) Reset() error {
	return leaveOneOut.median.Reset()
}

func (leaveOneOut *LeaveOneOutMedian) peerSamples(excludedKey float64) []float64 {
	peerSamples := make([]float64, 0)

	leaveOneOut.panel.values.Range(func(key, value any) bool {
		memberKey, keyOK := key.(float64)
		sample, valueOK := value.(float64)

		if !keyOK || !valueOK || memberKey == excludedKey {
			return true
		}

		peerSamples = append(peerSamples, sample)

		return true
	})

	return peerSamples
}

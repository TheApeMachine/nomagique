package statistic

import (
	"sync"

	"github.com/theapemachine/nomagique/core"
)

/*
Panel is a live registry of keyed numeric samples.

Each member key maps to that member's latest reading. The panel does not compute
a summary; it only remembers the latest value for every member so downstream
stages can aggregate across the collection.

Feed Panel through nomagique.Number(...) or call Observe with two inputs: member
key, then sample value. Keys are float64 identifiers. The returned value is the
sample that was just stored.
*/
type Panel[T ~float64] struct {
	values sync.Map
	output core.Scalar[T]
}

/*
Observe registers or updates one member's latest sample.

Inputs (in order):
 1. member key
 2. sample value

Fewer than two scalar inputs returns the prior output and stores nothing.
*/
func (panel *Panel[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	memberKey, sampleValue, ok := pairScalars(inputs...)

	if !ok {
		return panel.output
	}

	panel.values.Store(memberKey, sampleValue)
	panel.output = core.Scalar[T](T(sampleValue))

	return panel.output
}

/*
Reset clears every stored member sample.

Use when the member set rolls so stale keys do not leak into later reads.
*/
func (panel *Panel[T]) Reset() error {
	panel.values = sync.Map{}
	panel.output = core.Scalar[T](0)

	return nil
}

/*
LeaveOneOutMedian answers: "What is the typical peer reading if we ignore one
member?"

Given a Panel of member samples, it collects every value except the excluded
member key and passes those peer values to Median. This is the usual way to
estimate cross-section drift without letting one member vote on its own baseline.

Compose with nomagique.Number:

	panel := statistic.Panel[float64]{}
	leaveOneOut := statistic.NewLeaveOneOutMedian(&panel)
	macro := nomagique.Number(leaveOneOut)
*/
type LeaveOneOutMedian[T ~float64] struct {
	panel  *Panel[T]
	median *Median[T]
	output core.Scalar[T]
}

/*
NewLeaveOneOutMedian binds a Panel to a leave-one-out median stage.

The same Panel pointer must be shared between registration (Panel.Observe) and
aggregation (LeaveOneOutMedian.Observe). One panel can feed many composed
Numbers as long as they all reference the same underlying registry.
*/
func NewLeaveOneOutMedian[T ~float64](panel *Panel[T]) *LeaveOneOutMedian[T] {
	return &LeaveOneOutMedian[T]{
		panel:  panel,
		median: NewMedian[T](nil),
	}
}

/*
Observe returns the median of all panel members except the excluded key.

Input: the member key to leave out as the boundary scalar.

Returns zero when the panel is empty, the excluded key is unknown, or no peers
remain after exclusion.
*/
func (leaveOneOut *LeaveOneOutMedian[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	excludedKey, ok := boundarySample(inputs...)

	if !ok {
		return leaveOneOut.output
	}

	peerSamples := leaveOneOut.peerSamples(excludedKey)

	if len(peerSamples) == 0 {
		return leaveOneOut.output
	}

	peerNumbers := make([]core.Number[T], len(peerSamples))

	for index, sample := range peerSamples {
		peerNumbers[index] = core.Scalar[T](T(sample))
	}

	leaveOneOut.output = leaveOneOut.median.Observe(peerNumbers...)

	return leaveOneOut.output
}

/*
Reset clears derived median state on the embedded Median stage.

Panel member samples are not cleared here; call Panel.Reset when the universe
should forget prior registrations.
*/
func (leaveOneOut *LeaveOneOutMedian[T]) Reset() error {
	leaveOneOut.output = core.Scalar[T](0)

	return leaveOneOut.median.Reset()
}

func (leaveOneOut *LeaveOneOutMedian[T]) peerSamples(excludedKey float64) []float64 {
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

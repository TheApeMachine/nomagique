package excitation

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/theapemachine/nomagique/hawkes"
)

/*
forecastPendingReason documents why intensity can be ready while forecast gates stay closed.
*/
const forecastPendingReason = "forecast readiness requires residual and out-of-sample validation"

/*
symbol owns the adaptive observation workspace, fitted parameter epoch, and
evidence-derived invalidation schedule for one market. Ingestion evaluates with
the latest retained parameters; unrestricted and restricted fits run on an
immutable snapshot and publish a new epoch atomically.
*/
type symbol struct {
	model           hawkes.BivariateFit
	restricted      hawkes.BivariateFit
	fitPrior        hawkes.BivariateFit
	hasFit          bool
	fitObservedFrom time.Time
	fitAt           time.Time
	lastOutcome     Outcome
	lastReady       bool
	adaptive        *hawkes.ArrivalWorkspace
	revision        *revision
	schedule        *schedule
	mu              sync.Mutex
	fitting         atomic.Bool
	pending         *fitEpoch
	fitSignal       chan struct{}
}

/*
fitEpoch is an immutable publish bundle from one completed Hawkes estimation.
*/
type fitEpoch struct {
	model        hawkes.BivariateFit
	restricted   hawkes.BivariateFit
	prior        hawkes.BivariateFit
	observedFrom time.Time
	at           time.Time
	eventCount   int
}

/*
newSymbol wires adaptive workspace, revision counting, and async fit signaling.
*/
func newSymbol() *symbol {
	return &symbol{
		adaptive:  hawkes.NewArrivalWorkspace(),
		revision:  &revision{},
		schedule:  &schedule{},
		fitSignal: make(chan struct{}, 1),
	}
}

/*
measure ingests one observation horizon and returns Hawkes outcomes for envelope mapping.
*/
func (symbol *symbol) measure(
	stream hawkes.ArrivalStream,
	horizon time.Time,
) (Outcome, bool) {
	context, observed, ready := symbol.context(stream, horizon)

	if !ready {
		return symbol.retain(symbol.observation(stream, horizon)), true
	}

	changedEvents := symbol.revision.Observe(observed)

	if symbol.hasFit {
		symbol.schedule.Observe(changedEvents)
	}

	if epoch := symbol.takePending(); epoch != nil {
		return symbol.retain(symbol.publish(epoch, context, observed, horizon)), true
	}

	if !context.EnoughEvents(observed) {
		return symbol.retain(symbol.baseline(context, horizon)), true
	}

	if symbol.hasFit {
		outcome := symbol.project(context, observed, horizon)

		if symbol.schedule.Ready() {
			symbol.requestFit(context, observed, horizon)
		}

		return symbol.retain(outcome), true
	}

	epoch, ok := symbol.computeFit(context, observed, horizon, hawkes.BivariateFit{})

	if !ok {
		return Outcome{}, false
	}

	return symbol.retain(symbol.publish(&epoch, context, observed, horizon)), true
}

/*
retain caches the latest readable outcome for Process.Outcome queries.
*/
func (symbol *symbol) retain(outcome Outcome) Outcome {
	symbol.lastOutcome = outcome
	symbol.lastReady = true

	return outcome
}

/*
observation emits counts-only outcomes when the window is not yet fittable.
*/
func (symbol *symbol) observation(
	stream hawkes.ArrivalStream,
	horizon time.Time,
) Outcome {
	observedFrom := stream.ObservationOrigin()
	context, ready := hawkes.NewObservationContext(stream, horizon)

	if ready {
		return symbol.outcome(context, horizon, hawkes.BivariateFit{})
	}

	buyCount, sellCount := stream.ObservationCounts(horizon)

	if horizon.Equal(observedFrom) {
		for _, eventTime := range stream.BuyTimes() {
			if eventTime.Equal(horizon) {
				buyCount++
			}
		}

		for _, eventTime := range stream.SellTimes() {
			if eventTime.Equal(horizon) {
				sellCount++
			}
		}
	}

	return Outcome{
		ObservedFrom:   observedFrom,
		At:             horizon,
		Horizon:        horizon.Sub(observedFrom),
		EventCount:     buyCount + sellCount,
		BuyEventCount:  buyCount,
		SellEventCount: sellCount,
		Readiness: Readiness{
			Observation: true,
			Reason:      "arrival rate requires a positive observation interval",
		},
	}
}

/*
baseline projects Poisson intensities while Hawkes parameters are still pending.
*/
func (symbol *symbol) baseline(
	context hawkes.FitContext,
	horizon time.Time,
) Outcome {
	outcome := symbol.outcome(context, horizon, context.PoissonFit())
	outcome.Readiness = Readiness{
		Observation: true,
		Intensity:   true,
		Reason:      symbol.pendingReason(context),
	}

	return outcome
}

/*
pendingReason explains maturity gates so readiness Reason is actionable upstream.
*/
func (symbol *symbol) pendingReason(context hawkes.FitContext) string {
	buyCount := context.EventsX
	sellCount := context.EventsY

	if buyCount < context.MinPerSide || sellCount < context.MinPerSide {
		return fmt.Sprintf(
			"hawkes fit requires %d events per side; observed buy=%d sell=%d",
			context.MinPerSide,
			buyCount,
			sellCount,
		)
	}

	return fmt.Sprintf(
		"hawkes fit requires %d events; observed %d",
		context.MinFitEvents,
		buyCount+sellCount,
	)
}

/*
requestFit snapshots arrivals and fits asynchronously without blocking Measure.
*/
func (symbol *symbol) requestFit(
	context hawkes.FitContext,
	stream hawkes.ArrivalStream,
	horizon time.Time,
) {
	if !symbol.fitting.CompareAndSwap(false, true) {
		return
	}

	prior := symbol.model
	origin := stream.ObservationOrigin()
	buyTimes := append([]time.Time(nil), stream.BuyTimes()...)
	sellTimes := append([]time.Time(nil), stream.SellTimes()...)

	go func() {
		defer symbol.fitting.Store(false)

		snapshot := hawkes.NewArrivalStreamFrom(origin, buyTimes, sellTimes)
		epoch, ok := symbol.computeFit(context, snapshot, horizon, prior)

		if !ok {
			symbol.notifyFit()

			return
		}

		symbol.mu.Lock()
		symbol.pending = &epoch
		symbol.mu.Unlock()
		symbol.notifyFit()
	}()
}

/*
takePending moves a finished async fit out for publication. Caller holds symbol.mu
because Measure already owns that lock and the mutex must not reenter here.
*/
func (symbol *symbol) takePending() *fitEpoch {
	pending := symbol.pending
	symbol.pending = nil

	return pending
}

/*
notifyFit wakes AwaitFit waiters when a background fit completes or fails.
*/
func (symbol *symbol) notifyFit() {
	select {
	case symbol.fitSignal <- struct{}{}:
	default:
	}
}

/*
awaitFit blocks until an in-flight fit finishes and an epoch is pending publish.
*/
func (symbol *symbol) awaitFit() bool {
	if !symbol.fitting.Load() && symbol.peekPending() {
		return true
	}

	if !symbol.fitting.Load() {
		return symbol.peekPending()
	}

	<-symbol.fitSignal

	return symbol.peekPending() || !symbol.fitting.Load()
}

/*
peekPending reports whether a completed epoch waits without consuming it.
*/
func (symbol *symbol) peekPending() bool {
	symbol.mu.Lock()
	defer symbol.mu.Unlock()

	return symbol.pending != nil
}

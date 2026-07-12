package excitation

import (
	"fmt"
	"math"
	"time"

	"github.com/theapemachine/nomagique/hawkes"
)

const forecastPendingReason = "forecast readiness requires residual and out-of-sample validation"

/*
symbol owns the adaptive observation workspace, fitted parameter epoch, and
evidence-derived invalidation schedule for one market. Keeping these together
lets projections use the exact retained fit without pretending its parameters
were re-estimated.
*/
type symbol struct {
	model           hawkes.BivariateFit
	hasFit          bool
	fitObservedFrom time.Time
	fitAt           time.Time
	adaptive        *hawkes.ArrivalWorkspace
	revision        *revision
	schedule        *schedule
}

func newSymbol() *symbol {
	return &symbol{
		adaptive: hawkes.NewArrivalWorkspace(),
		revision: &revision{},
		schedule: &schedule{},
	}
}

func (symbol *symbol) measure(
	stream hawkes.ArrivalStream,
	horizon time.Time,
) (Outcome, bool) {
	context, observed, ready := symbol.context(stream, horizon)

	if !ready {
		return symbol.observation(stream, horizon), true
	}

	changedEvents := symbol.revision.Observe(observed)

	if symbol.hasFit {
		symbol.schedule.Observe(changedEvents)
	}

	if !context.EnoughEvents(observed) {
		return symbol.baseline(context, observed, horizon), true
	}

	if symbol.hasFit && !symbol.schedule.Ready() {
		return symbol.project(context, observed, horizon), true
	}

	return symbol.fit(context, observed, horizon)
}

func (symbol *symbol) observation(
	stream hawkes.ArrivalStream,
	horizon time.Time,
) Outcome {
	observedFrom, _, _ := stream.Bounds()
	buyCount := len(stream.BuyTimes())
	sellCount := len(stream.SellTimes())

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

func (symbol *symbol) baseline(
	context hawkes.FitContext,
	stream hawkes.ArrivalStream,
	horizon time.Time,
) Outcome {
	outcome := symbol.outcome(
		context,
		stream,
		horizon,
		hawkes.BivariateFit{},
	)
	outcome.Readiness = Readiness{
		Observation: true,
		Intensity:   true,
		Reason:      symbol.pendingReason(context, stream),
	}

	return outcome
}

func (symbol *symbol) pendingReason(
	context hawkes.FitContext,
	stream hawkes.ArrivalStream,
) string {
	buyCount := len(stream.BuyTimes())
	sellCount := len(stream.SellTimes())

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

func (symbol *symbol) fit(
	context hawkes.FitContext,
	stream hawkes.ArrivalStream,
	horizon time.Time,
) (Outcome, bool) {
	prior := hawkes.BivariateFit{}

	if symbol.hasFit {
		prior = symbol.model
	}

	estimator := hawkes.NewBivariateEstimator(prior)
	fit := estimator.Fit(stream, horizon)

	if !fit.Valid() {
		return Outcome{}, false
	}

	selfOnly := estimator.FitSelfOnly(stream, horizon)

	if !selfOnly.Valid() {
		return Outcome{}, false
	}

	fullLikelihood := fit.LogLikelihood(stream, horizon)
	selfLikelihood := selfOnly.LogLikelihood(stream, horizon)

	if selfLikelihood > fullLikelihood {
		fit = selfOnly
		fullLikelihood = selfLikelihood
	}

	immediateBuy, immediateSell, immediateReady := fit.ImmediateOffspring()
	totalBuy, totalSell, totalReady := fit.TotalDescendants()

	if !immediateReady || !totalReady {
		return Outcome{}, false
	}

	poisson := context.PoissonFit().WithIntensitiesAt(stream, horizon)
	outcome := symbol.outcome(context, stream, horizon, fit)
	outcome.HawkesPoissonLogLikelihoodDelta =
		fullLikelihood - poisson.LogLikelihood(stream, horizon)
	outcome.CrossSelfLogLikelihoodDelta =
		fullLikelihood - selfLikelihood
	outcome.ImmediateBuyOffspring = immediateBuy
	outcome.ImmediateSellOffspring = immediateSell
	outcome.TotalBuyDescendants = totalBuy
	outcome.TotalSellDescendants = totalSell
	outcome.Maturity = 1
	outcome.Readiness = Readiness{
		Observation:  true,
		Intensity:    true,
		HawkesFit:    true,
		ModelUpdated: true,
		Reason:       forecastPendingReason,
	}
	symbol.model = fit
	symbol.hasFit = true
	symbol.fitObservedFrom = outcome.ObservedFrom
	symbol.fitAt = outcome.At
	symbol.schedule.Reset(outcome.EventCount)
	outcome.FitObservedFrom = symbol.fitObservedFrom
	outcome.FitAt = symbol.fitAt

	return outcome, true
}

func (symbol *symbol) project(
	context hawkes.FitContext,
	stream hawkes.ArrivalStream,
	horizon time.Time,
) Outcome {
	fit := symbol.model.WithIntensitiesAt(stream, horizon)
	outcome := symbol.outcome(context, stream, horizon, fit)
	outcome.FitObservedFrom = symbol.fitObservedFrom
	outcome.FitAt = symbol.fitAt
	outcome.Readiness = Readiness{
		Observation: true,
		Intensity:   true,
		HawkesFit:   true,
		Reason: fmt.Sprintf(
			"conditional intensity uses retained fit; %d changed events remain before refit; %s",
			symbol.schedule.Remaining(),
			forecastPendingReason,
		),
	}

	return outcome
}

func (symbol *symbol) outcome(
	context hawkes.FitContext,
	stream hawkes.ArrivalStream,
	horizon time.Time,
	fit hawkes.BivariateFit,
) Outcome {
	observedFrom, _, _ := stream.Bounds()
	buyCount := len(stream.BuyTimes())
	sellCount := len(stream.SellTimes())
	eventCount := buyCount + sellCount
	maturity := math.Min(
		float64(eventCount)/float64(context.MinFitEvents),
		math.Min(
			float64(buyCount)/float64(context.MinPerSide),
			float64(sellCount)/float64(context.MinPerSide),
		),
	)
	span := horizon.Sub(observedFrom)
	buyRate := float64(buyCount) / span.Seconds()
	sellRate := float64(sellCount) / span.Seconds()

	if maturity > 1 {
		maturity = 1
	}

	return Outcome{
		Fit:              fit,
		ObservedFrom:     observedFrom,
		At:               horizon,
		Horizon:          horizon.Sub(observedFrom),
		EventCount:       eventCount,
		BuyEventCount:    buyCount,
		SellEventCount:   sellCount,
		BuyArrivalRate:   buyRate,
		SellArrivalRate:  sellRate,
		MinimumFitEvents: context.MinFitEvents,
		Maturity:         maturity,
	}
}

func (symbol *symbol) context(
	stream hawkes.ArrivalStream,
	horizon time.Time,
) (hawkes.FitContext, hawkes.ArrivalStream, bool) {
	context, ready := hawkes.NewObservationContext(stream, horizon)

	if !ready {
		return hawkes.FitContext{}, hawkes.ArrivalStream{}, false
	}

	observed := stream.BetweenInto(
		horizon.Add(-context.TradeWindow),
		horizon,
		symbol.adaptive,
	)

	if len(observed.BuyTimes()) == len(stream.BuyTimes()) &&
		len(observed.SellTimes()) == len(stream.SellTimes()) {
		return context, stream, true
	}

	context, ready = hawkes.NewObservationContext(observed, horizon)

	return context, observed, ready
}

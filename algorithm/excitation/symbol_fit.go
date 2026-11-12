package excitation

import (
	"fmt"
	"math"
	"time"

	"github.com/theapemachine/nomagique/hawkes"
)

/*
computeFit runs bivariate and self-only estimators and rejects fits without offspring readiness.
*/
func (symbol *symbol) computeFit(
	context hawkes.FitContext,
	stream hawkes.ArrivalStream,
	horizon time.Time,
	prior hawkes.BivariateFit,
) (fitEpoch, bool) {
	estimator := hawkes.NewBivariateEstimator(prior)
	fit := estimator.Fit(stream, horizon)

	if !fit.Valid() {
		return fitEpoch{}, false
	}

	selfOnly := estimator.FitSelfOnly(stream, horizon)

	if !selfOnly.Valid() {
		return fitEpoch{}, false
	}

	fullLikelihood := fit.LogLikelihood(stream, horizon)
	selfLikelihood := selfOnly.LogLikelihood(stream, horizon)

	if selfLikelihood > fullLikelihood {
		fit = selfOnly
	}

	_, _, immediateReady := fit.ImmediateOffspring()
	_, _, totalReady := fit.TotalDescendants()

	if !immediateReady || !totalReady {
		return fitEpoch{}, false
	}

	return fitEpoch{
		model:        fit,
		restricted:   selfOnly,
		prior:        prior,
		observedFrom: context.ObservedFrom,
		at:           horizon,
		eventCount:   context.TotalEvents,
	}, true
}

/*
publish commits a new parameter epoch and resets the event-driven refit schedule.
*/
func (symbol *symbol) publish(
	epoch *fitEpoch,
	context hawkes.FitContext,
	stream hawkes.ArrivalStream,
	horizon time.Time,
) Outcome {
	symbol.fitPrior = epoch.prior
	symbol.model = epoch.model
	symbol.restricted = epoch.restricted
	symbol.hasFit = true
	symbol.fitObservedFrom = epoch.observedFrom
	symbol.fitAt = epoch.at
	symbol.schedule.Reset(epoch.eventCount)

	outcome := symbol.evaluate(
		context,
		stream,
		horizon,
		epoch.model,
		epoch.restricted,
		true,
	)
	outcome.FitObservedFrom = symbol.fitObservedFrom
	outcome.FitAt = symbol.fitAt

	return outcome
}

/*
project evaluates the retained fit between refits and surfaces remaining schedule events.
*/
func (symbol *symbol) project(
	context hawkes.FitContext,
	stream hawkes.ArrivalStream,
	horizon time.Time,
) Outcome {
	outcome := symbol.evaluate(
		context,
		stream,
		horizon,
		symbol.model,
		symbol.restricted,
		false,
	)
	outcome.FitObservedFrom = symbol.fitObservedFrom
	outcome.FitAt = symbol.fitAt
	outcome.Readiness.Reason = fmt.Sprintf(
		"conditional intensity uses retained fit; %d changed events remain before refit; %s",
		symbol.schedule.Remaining(),
		forecastPendingReason,
	)

	return outcome
}

/*
evaluate attaches likelihood deltas and offspring metrics to the base outcome shell.
*/
func (symbol *symbol) evaluate(
	context hawkes.FitContext,
	stream hawkes.ArrivalStream,
	horizon time.Time,
	model hawkes.BivariateFit,
	restricted hawkes.BivariateFit,
	modelUpdated bool,
) Outcome {
	fit := model.WithIntensitiesAt(stream, horizon)
	fullLikelihood := fit.LogLikelihood(stream, horizon)
	selfLikelihood := restricted.WithIntensitiesAt(stream, horizon).
		LogLikelihood(stream, horizon)
	poisson := context.PoissonFit().WithIntensitiesAt(stream, horizon)
	immediateBuy, immediateSell, _ := model.ImmediateOffspring()
	totalBuy, totalSell, _ := model.TotalDescendants()
	outcome := symbol.outcome(context, horizon, fit)
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
		ModelUpdated: modelUpdated,
		Reason:       forecastPendingReason,
	}

	return outcome
}

/*
outcome builds the portable Hawkes reading symm maps into envelope features.
*/
func (symbol *symbol) outcome(
	context hawkes.FitContext,
	horizon time.Time,
	fit hawkes.BivariateFit,
) Outcome {
	observedFrom := context.ObservedFrom
	buyCount := context.EventsX
	sellCount := context.EventsY
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

/*
context trims the observation window to the adaptive trade horizon before fitting.
*/
func (symbol *symbol) context(
	stream hawkes.ArrivalStream,
	horizon time.Time,
) (hawkes.FitContext, hawkes.ArrivalStream, bool) {
	context, ready := hawkes.NewObservationContext(stream, horizon)

	if !ready {
		return hawkes.FitContext{}, hawkes.ArrivalStream{}, false
	}

	origin := horizon.Add(-context.TradeWindow)

	if origin.Before(stream.ObservationOrigin()) {
		origin = stream.ObservationOrigin()
	}

	observed := stream.WithObservationOrigin(origin)

	context, ready = hawkes.NewObservationContext(observed, horizon)

	return context, observed, ready
}

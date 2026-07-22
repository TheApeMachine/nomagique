package algorithm

import (
	"math"
	"sync"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/adaptive"
	"github.com/theapemachine/nomagique/algorithm/book/flow"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/statistic"
)

/*
decayWindow tracks one symbol's microstructure state for Decay. Depth
trend, spread deviation, pressure fade, and imbalance flip are each backed
by an adaptive nomagique primitive, so every quantity is defined from its
first observation instead of gating on a minimum sample count.

Depth and spread tracking use adaptive.Variance rather than retained history
arrays. Its online mean and variance keep O(1) space and time without an
externally chosen window. statistic.Max, Min, and Mean are also O(1).
*/
type decayWindow struct {
	book               *flow.Book
	bidDepthVariance   *adaptive.Variance
	askDepthVariance   *adaptive.Variance
	densityVariance    *adaptive.Variance
	spreadVariance     *adaptive.Variance
	imbalanceMean      *statistic.Mean
	priorImbalanceMean float64
	pressurePeak       *statistic.Max
	pressureTrough     *statistic.Min
	lastFeatures       equation.DecayInput
	tradePressure      float64
	tradeFrames        int
	lastPrice          float64
	observations       int
	mu                 sync.Mutex
}

/*
newDecayWindow constructs one symbol-local set of online statistics.
*/
func newDecayWindow() *decayWindow {
	return &decayWindow{
		book:             flow.NewBook(),
		bidDepthVariance: adaptive.NewVariance(),
		askDepthVariance: adaptive.NewVariance(),
		densityVariance:  adaptive.NewVariance(),
		spreadVariance:   adaptive.NewVariance(),
		imbalanceMean:    statistic.NewMean(),
		pressurePeak:     statistic.NewMax(),
		pressureTrough:   statistic.NewMin(),
	}
}

/*
ingestBook reconstructs the symbol book and records one lattice-normalized
microstructure observation.
*/
func (window *decayWindow) ingestBook(input flow.BookInput) error {
	if err := window.book.Configure(input); err != nil {
		return err
	}

	if _, err := window.book.ApplyLevels(input.Bids, flow.SideBid); err != nil {
		return err
	}

	if _, err := window.book.ApplyLevels(input.Asks, flow.SideAsk); err != nil {
		return err
	}

	mid := window.book.Mid()
	spread := window.book.Spread()

	// Crossed or one-sided venues do not establish decay microstructure. Leave
	// the feature cache untouched; MeasureBook reports not-ready for this cut.
	if mid <= 0 || spread <= 0 {
		return nil
	}

	// Quotes occupy the venue tick lattice, so their midpoint occupies its
	// half-tick lattice and their spread occupies whole ticks. Normalize before
	// differencing so binary float residue cannot become decay evidence.
	mid = math.Round(2*mid/input.TickSize) * input.TickSize / 2
	spread = math.Round(spread/input.TickSize) * input.TickSize

	priceReturn := 0.0

	if window.lastPrice > 0 {
		priceReturn = math.Log(mid / window.lastPrice)
	}

	window.lastPrice = mid
	bidDepth := window.book.SideDepth(flow.SideBid)
	askDepth := window.book.SideDepth(flow.SideAsk)
	decayRate := flow.DecayRate(mid, spread)
	imbalance := window.book.Imbalance(mid, decayRate, false, 0, 0, 0)

	return window.observe(bidDepth, askDepth, spread, imbalance, priceReturn)
}

/*
ingestTrade incorporates aggressor notional immediately so a trade event never
re-emits stale pressure while waiting for another book update.
*/
func (window *decayWindow) ingestTrade(input flow.TradeInput) error {
	notional := input.Price * input.Quantity
	signedNotional := notional

	if input.Side == "sell" {
		signedNotional = -notional
	}

	window.tradeFrames++
	smoothing := 2.0 / float64(window.tradeFrames+1)

	if smoothing > 1 {
		smoothing = 1
	}

	window.tradePressure += smoothing * (signedNotional - window.tradePressure)
	pressurePeak, pressureTrough, err := window.observePressure()

	if err != nil {
		return err
	}

	window.lastFeatures.PriceReturn = 0
	window.lastFeatures.Pressure = window.tradePressure
	window.lastFeatures.PressurePeak = pressurePeak
	window.lastFeatures.PressureTrough = pressureTrough

	if window.lastPrice <= 0 {
		window.lastPrice = input.Price
	}

	return nil
}

/*
observe feeds one new book-derived reading into every adaptive statistic
and refreshes the window's cached feature snapshot.
*/
func (window *decayWindow) observe(
	bidDepth, askDepth, spread, imbalance, priceReturn float64,
) error {
	bidRatio, err := window.ratio(window.bidDepthVariance, bidDepth, "bid depth ratio")

	if err != nil {
		return err
	}

	askRatio, err := window.ratio(window.askDepthVariance, askDepth, "ask depth ratio")

	if err != nil {
		return err
	}

	densityRatio, err := window.ratio(window.densityVariance, bidDepth+askDepth, "density ratio")

	if err != nil {
		return err
	}

	spreadDeviation, err := window.deviation(spread)

	if err != nil {
		return err
	}

	priorImbalanceMean, err := window.observeImbalance(imbalance)

	if err != nil {
		return err
	}

	window.observations++
	window.lastFeatures = equation.DecayInput{
		PriceReturn:        priceReturn,
		BidDepthRatio:      bidRatio,
		AskDepthRatio:      askRatio,
		DensityRatio:       densityRatio,
		SpreadDeviation:    spreadDeviation,
		Pressure:           window.tradePressure,
		PressurePeak:       window.lastFeatures.PressurePeak,
		PressureTrough:     window.lastFeatures.PressureTrough,
		Imbalance:          imbalance,
		PriorImbalanceMean: priorImbalanceMean,
	}

	return nil
}

/*
ratio reports the current sample over the series' own adaptive mean. Below
1 means the current reading sits under its own recent baseline.
*/
func (window *decayWindow) ratio(
	tracker *adaptive.Variance, sample float64, what string,
) (float64, error) {
	output, err := tracker.Measure(sample)

	if err != nil {
		return 0, decayWindowErr(what, err)
	}

	if output.Mean <= 0 {
		return 0, decayWindowErr(what+" baseline must be positive", nil)
	}

	return sample / output.Mean, nil
}

/*
deviation reports the current spread's z-score against its own adaptive
mean and variance. It falls back to a sign-only reading when the variance
is indeterminate, mirroring statistic.RollingZScore's own graceful
degradation without needing retained history to fall back on a
mean-absolute-deviation.
*/
func (window *decayWindow) deviation(spread float64) (float64, error) {
	output, err := window.spreadVariance.Measure(spread)

	if err != nil {
		return 0, decayWindowErr("spread deviation", err)
	}

	if !output.Ready || output.Value <= 0 {
		delta := spread - output.Mean

		if delta == 0 {
			return 0, nil
		}

		return delta / math.Abs(delta), nil
	}

	return (spread - output.Mean) / math.Sqrt(output.Value), nil
}

/*
observePressure updates the directional extrema used to measure later fade.
*/
func (window *decayWindow) observePressure() (peak, trough float64, err error) {
	peakOutput, err := window.pressurePeak.Measure(window.tradePressure)

	if err != nil {
		return 0, 0, decayWindowErr("pressure peak", err)
	}

	troughOutput, err := window.pressureTrough.Measure(window.tradePressure)

	if err != nil {
		return 0, 0, decayWindowErr("pressure trough", err)
	}

	return peakOutput.Value, troughOutput.Value, nil
}

/*
observeImbalance returns the mean of every imbalance observed strictly
before this tick, then folds the current reading into the running mean for
the next tick.
*/
func (window *decayWindow) observeImbalance(imbalance float64) (float64, error) {
	priorMean := window.priorImbalanceMean
	output, err := window.imbalanceMean.Measure(imbalance)

	if err != nil {
		return 0, decayWindowErr("imbalance mean", err)
	}

	window.priorImbalanceMean = output.Value

	return priorMean, nil
}

/*
features returns a scoreable input only after a book established all depth
ratios; trade prices alone cannot stand in for missing microstructure.
*/
func (window *decayWindow) features() (equation.DecayInput, bool, float64, error) {
	if window.lastPrice <= 0 || window.observations == 0 {
		return equation.DecayInput{}, false, 0, nil
	}

	output := window.lastFeatures
	output.LastPrice = window.lastPrice

	return output, true, window.maturity(), nil
}

/*
maturity reports a monotonically increasing, asymptotic confidence in the
window's adaptive statistics as more observations accumulate. It never
reaches exactly 1 and never gates emission — the window already emits a
defined value from the first observation — it only communicates how much
independent evidence backs that value so far.
*/
func (window *decayWindow) maturity() float64 {
	observations := float64(window.observations)

	return observations / (observations + 1)
}

/*
decayWindowErr gives every online-statistic failure its feature context.
*/
func decayWindowErr(what string, err error) error {
	return errnie.Error(errnie.Err(
		errnie.Validation,
		"decay-window: "+what+" failed",
		err,
	))
}

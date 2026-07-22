package equation

import (
	"math"
	"time"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/statistic"
)

/*
ignitionWindow owns the bounded volume-clock history and held scores for one
symbol. Its target quantities, normalizers, and counter-evidence scales all
come from retained observations from that same symbol.
*/
type ignitionWindow struct {
	capacity int

	initialized bool
	classified  bool
	bars        int
	lastVolume  float64
	lastTime    time.Time
	haveTime    bool
	barVolume   float64
	barOpenTime time.Time
	prevClose   float64
	lastRVOL    float64

	deltas     []float64
	rates      []float64
	returns    []float64
	precursors []float64
	spreads    []float64
	cached     IgnitionOutput
}

/*
observe validates and advances one symbol without mixing its state into the
public multi-symbol coordinator.
*/
func (window *ignitionWindow) observe(
	input IgnitionInput,
) (IgnitionOutput, bool, float64, error) {
	spread := input.Ask - input.Bid

	if !window.initialized {
		window.initialized = true
		window.lastVolume = input.Volume
		window.prevClose = input.Last
		window.lastTime = input.At
		window.barOpenTime = input.At
		window.haveTime = !input.At.IsZero()
		window.spreads = window.appendPositive(window.spreads, spread)

		return window.compose(spread), window.ready(), window.maturity(), nil
	}

	if input.Volume < window.lastVolume {
		return IgnitionOutput{}, false, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"ignition: cumulative executed volume cannot decrease",
			nil,
		))
	}

	if window.haveTime && !input.At.IsZero() && input.At.Before(window.lastTime) {
		return IgnitionOutput{}, false, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"ignition: observation time cannot move backwards",
			nil,
		))
	}

	if err := window.advance(input, spread); err != nil {
		return IgnitionOutput{}, false, 0, err
	}

	window.spreads = window.appendPositive(window.spreads, spread)

	return window.compose(spread), window.ready(), window.maturity(), nil
}

/*
advance accumulates executed quantity, advances event time, and closes at most
one indivisible observation into the empirical volume clock.
*/
func (window *ignitionWindow) advance(input IgnitionInput, spread float64) error {
	delta := input.Volume - window.lastVolume
	window.lastVolume = input.Volume

	if !input.At.IsZero() {
		window.lastTime = input.At

		if !window.haveTime {
			window.haveTime = true
			window.barOpenTime = input.At
		}
	}

	if delta > 0 {
		window.barVolume += delta
		window.deltas = window.appendPositive(window.deltas, delta)
	}

	barTarget, targetReady := statistic.MedianOf(window.deltas)

	if !targetReady || barTarget <= 0 || window.barVolume < barTarget ||
		!window.haveTime || !input.At.After(window.barOpenTime) {
		return nil
	}

	if err := window.closeBar(
		input.Last,
		input.At,
		window.barVolume,
		spread,
	); err != nil {
		return err
	}

	window.barVolume = 0

	return nil
}

/*
closeBar scores an empirical volume bar, then retains it for later baselines.
*/
func (window *ignitionWindow) closeBar(
	closePrice float64,
	at time.Time,
	barVolume float64,
	spread float64,
) error {
	priceMove := math.Log(closePrice / window.prevClose)
	duration := at.Sub(window.barOpenTime).Seconds()

	if duration <= 0 {
		return errnie.Error(errnie.Err(
			errnie.Validation,
			"ignition: volume bar requires positive elapsed event time",
			nil,
		))
	}

	barRate := barVolume / duration

	if err := window.score(barRate, priceMove, spread); err != nil {
		return err
	}

	window.rates = window.appendPositive(window.rates, barRate)
	window.returns = window.appendNonNegative(window.returns, math.Abs(priceMove))
	window.precursors = window.appendPositive(window.precursors, priceMove)
	window.prevClose = closePrice
	window.barOpenTime = at
	window.bars++

	return nil
}

/*
score derives one held event classification from prior empirical baselines. A
dependent score remains zero when any scale it needs has not yet been observed;
calculation failures are returned instead of being converted into evidence.
*/
func (window *ignitionWindow) score(
	barRate float64,
	priceMove float64,
	spread float64,
) error {
	rateBaseline, rateReady := statistic.MedianOf(window.rates)
	spreadBaseline, spreadReady := statistic.MedianOf(window.spreads)
	precursorBaseline, precursorReady := statistic.MedianOf(window.precursors)
	moveBaseline, moveReady := statistic.MedianOf(window.returns)

	rvol := ignitionRatio(barRate, rateBaseline, rateReady)
	precursor := ignitionRatio(math.Max(0, priceMove), precursorBaseline, precursorReady)
	compression := 0.0

	if spreadReady && spreadBaseline > 0 {
		compression = math.Max(0, 1-spread/spreadBaseline)
	}

	rejection := ignitionRatio(math.Max(0, -priceMove), moveBaseline, moveReady)
	output, err := ignitionFamilies(
		rvol,
		precursor,
		compression,
		ignitionRatioScale(window.rates, rateBaseline),
		ignitionRatioScale(window.precursors, precursorBaseline),
		ignitionCompressionScale(window.spreads, spreadBaseline),
	)

	if err != nil {
		return err
	}

	output.Exhaustion = ignitionExhaustion(
		window.lastRVOL,
		rvol,
		rejection,
		ignitionRatioScale(window.returns, moveBaseline),
	)
	output.Strength = max(
		output.Ignition,
		output.Compression,
		output.Trend,
		output.Exhaustion,
	)
	output.Value = output.Strength

	if math.IsNaN(output.Strength) || math.IsInf(output.Strength, 0) {
		return errnie.Error(errnie.Err(
			errnie.Validation,
			"ignition: calculated strength must be finite",
			nil,
		))
	}

	window.lastRVOL = rvol
	window.cached = output
	window.classified = rateReady && spreadReady

	return nil
}

/*
compose overlays the current executable spread on the last closed-bar scores.
*/
func (window *ignitionWindow) compose(spread float64) IgnitionOutput {
	output := window.cached
	output.Spread = spread

	return output
}

/*
ready reports that a causal volume bar and live spread history exist.
*/
func (window *ignitionWindow) ready() bool {
	return window.classified
}

/*
maturity reports a bounded closed-bar ratio without a configured horizon.
*/
func (window *ignitionWindow) maturity() float64 {
	return float64(window.bars) / float64(window.bars+1)
}

/*
appendPositive retains a finite positive baseline sample.
*/
func (window *ignitionWindow) appendPositive(
	values []float64,
	value float64,
) []float64 {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return values
	}

	return window.append(values, value)
}

/*
appendNonNegative retains a finite baseline sample where zero is meaningful.
*/
func (window *ignitionWindow) appendNonNegative(
	values []float64,
	value float64,
) []float64 {
	if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return values
	}

	return window.append(values, value)
}

/*
append bounds retained history to the market feed's explicit capacity.
*/
func (window *ignitionWindow) append(values []float64, value float64) []float64 {
	if len(values) < window.capacity {
		return append(values, value)
	}

	copy(values, values[1:])
	values[len(values)-1] = value

	return values
}

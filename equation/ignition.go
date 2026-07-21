package equation

import (
	"math"
	"time"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/probability"
	"github.com/theapemachine/nomagique/statistic"
)

/*
barTargetSeconds sets the volume-clock granularity: an equal-volume bar closes
once it has accumulated roughly this many seconds of *typical* executed volume.
It controls bar cadence and lookback only — the normalized rvol intensity divides
the threshold back out, so this constant never leaks into a reported score.
*/
const barTargetSeconds = 1.0

/*
Ignition scores ticker lift, price precursor, spread compression, and exhaustion
on a volume clock. Baselines are sampled per equal-volume bar rather than per
ticker advance, so quote churn that carries no executed volume cannot inject
zero-move samples into the calibration.
*/
type Ignition struct {
	windows  map[string]*ignitionWindow
	capacity int
	alpha    float64
}

/*
IgnitionInput is one ticker observation. Volume is the cumulative executed
quantity for the symbol; At is the observation time that drives the volume-clock
intensity. A zero At degrades the intensity signal but still forms price bars.
*/
type IgnitionInput struct {
	Symbol string
	Volume float64
	Last   float64
	Bid    float64
	Ask    float64
	At     time.Time
}

/*
IgnitionOutput contains direct ticker ignition scores. Spread is the live quote
spread refreshed every observation; the remaining scores are held from the most
recent closed volume bar so a calm quote-only tick keeps reporting real evidence.
*/
type IgnitionOutput struct {
	Value       float64
	RVOL        float64
	Precursor   float64
	Spread      float64
	Compression float64
	Ignition    float64
	Trend       float64
	Exhaustion  float64
	Strength    float64
	Category    float64
}

type ignitionWindow struct {
	capacity int
	alpha    float64

	initialized bool
	bars        int

	// Volume-clock accumulation.
	lastVolume   float64
	lastTime     time.Time
	haveTime     bool
	ewmaRate     float64
	barVolume    float64
	barOpenTime  time.Time
	prevClose    float64
	lastRVOL     float64

	// Per-bar baselines, bounded by the caller's retention capacity.
	rates      []float64 // executed volume per second, one sample per bar
	returns    []float64 // absolute log return per bar
	precursors []float64 // upward log return per bar
	spreads    []float64 // live quote spread, one sample per observation

	// Scores held from the most recent closed bar (Spread is applied live).
	cached IgnitionOutput
}

/*
NewIgnition returns a volume-clock ignition calculator whose per-symbol baseline
history is bounded by the caller's market-feed retention capacity.
*/
func NewIgnition(capacity int) *Ignition {
	alpha := 0.0

	if capacity > 0 {
		alpha = 2 / (float64(capacity) + 1)
	}

	return &Ignition{
		windows:  map[string]*ignitionWindow{},
		capacity: capacity,
		alpha:    alpha,
	}
}

/*
Measure scores one ticker observation and reports a bar-calibration maturity
alongside it. It never blanks the whole output: the live spread is always
reported once a quote exists, and each event score is gated only on its own
baseline rather than on a shared calibration flag.
*/
func (ignition *Ignition) Measure(
	input IgnitionInput,
) (IgnitionOutput, bool, float64, error) {
	if ignition == nil || ignition.capacity <= 0 {
		return IgnitionOutput{}, false, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"ignition: positive baseline capacity required",
			nil,
		))
	}

	if input.Symbol == "" || input.Volume <= 0 || input.Last <= 0 || input.Bid <= 0 || input.Ask <= 0 {
		return IgnitionOutput{}, false, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"ignition: symbol, volume, last, bid, and ask required",
			nil,
		))
	}

	if input.Ask <= input.Bid {
		return IgnitionOutput{}, false, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"ignition: ask must be above bid",
			nil,
		))
	}

	window := ignition.window(input.Symbol)
	spread := input.Ask - input.Bid
	window.spreads = window.appendPositive(window.spreads, spread)

	if !window.initialized {
		window.initialized = true
		window.lastVolume = input.Volume
		window.prevClose = input.Last

		if !input.At.IsZero() {
			window.lastTime = input.At
			window.barOpenTime = input.At
			window.haveTime = true
		}

		return window.compose(spread), window.ready(), window.maturity(), nil
	}

	delta := math.Max(0, input.Volume-window.lastVolume)
	window.lastVolume = input.Volume

	if window.haveTime && !input.At.IsZero() {
		if dt := input.At.Sub(window.lastTime).Seconds(); dt > 0 && delta > 0 {
			instant := delta / dt

			if window.ewmaRate <= 0 {
				window.ewmaRate = instant
			} else {
				window.ewmaRate += window.alpha * (instant - window.ewmaRate)
			}
		}
	}

	if !input.At.IsZero() {
		window.lastTime = input.At

		if !window.haveTime {
			window.haveTime = true
			window.barOpenTime = input.At
		}
	}

	window.barVolume += delta
	threshold := window.ewmaRate * barTargetSeconds

	// Close at most one bar per observation so each closed bar spans a distinct
	// timestamp; a burst that overfills the bar drains across the next few calls.
	if threshold > 0 && window.barVolume >= threshold && window.haveTime {
		window.closeBar(input.Last, input.At, threshold, spread)
		window.barVolume -= threshold
	}

	return window.compose(spread), window.ready(), window.maturity(), nil
}

func (ignition *Ignition) window(symbol string) *ignitionWindow {
	existing, ok := ignition.windows[symbol]

	if ok {
		return existing
	}

	window := &ignitionWindow{capacity: ignition.capacity, alpha: ignition.alpha}
	ignition.windows[symbol] = window

	return window
}

/*
closeBar finalizes one equal-volume bar: it records the bar's return and
intensity, refreshes the baselines, and recomputes the held event scores. Each
score is gated only on the inputs it needs, so a bar with a flat return still
yields a valid intensity while a calm baseline never zeros the spread.
*/
func (window *ignitionWindow) closeBar(
	closePrice float64,
	at time.Time,
	barVolume float64,
	spread float64,
) {
	priceMove := 0.0

	if window.prevClose > 0 && closePrice > 0 {
		priceMove = math.Log(closePrice / window.prevClose)
		window.returns = window.appendNonNegative(window.returns, math.Abs(priceMove))
		window.precursors = window.appendPositive(window.precursors, priceMove)
	}

	barRate := 0.0

	if dt := at.Sub(window.barOpenTime).Seconds(); dt > 0 {
		barRate = barVolume / dt
		window.rates = window.appendNonNegative(window.rates, barRate)
	}

	window.prevClose = closePrice
	window.barOpenTime = at
	window.bars++
	window.score(barRate, priceMove, spread)
}

/*
score derives the held event scores from the current baselines and the bar that
just closed. Missing baselines collapse only their own dependent scores.
*/
func (window *ignitionWindow) score(
	barRate float64,
	priceMove float64,
	spread float64,
) {
	rateBaseline, rateReady := statistic.MedianOf(window.rates)
	spreadBaseline, spreadReady := statistic.MedianOf(window.spreads)
	precursorBaseline, _ := statistic.MedianOf(window.precursors)
	moveBaseline, _ := statistic.MedianOf(window.returns)

	rvol := 0.0

	if rateReady && rateBaseline > 0 {
		rvol = barRate / rateBaseline
	}

	precursor := 0.0

	if precursorBaseline > 0 {
		precursor = math.Max(0, priceMove) / precursorBaseline
	}

	compression := 0.0

	if spreadReady && spreadBaseline > 0 {
		compression = math.Max(0, 1-spread/spreadBaseline)
	}

	rejection := 0.0

	if moveBaseline > 0 {
		rejection = math.Max(0, -priceMove) / moveBaseline
	}

	rvolScale := ratioScale(window.rates, rateBaseline)
	precursorScale := ratioScale(window.precursors, precursorBaseline)
	moveScale := ratioScale(window.returns, moveBaseline)
	compressionScale := compressionRatioScale(window.spreads, spreadBaseline)

	ignitionScore := geomean(rvol > 0 && precursor > 0, rvol, precursor)
	trendScore := geomean(rvol > 0 && precursor > 0,
		precursor,
		squashPositive(rvol, rvolScale),
		inverseSquash(compression, compressionScale),
	)
	compressionScore := geomean(compression > 0 && rvol > 0,
		compression,
		squashPositive(rvol, rvolScale),
		inverseSquash(precursor, precursorScale),
	)

	exhaustionScore := 0.0

	if window.lastRVOL > 0 && rejection > 0 {
		exhaustionScore = (math.Max(0, window.lastRVOL-rvol) / window.lastRVOL) *
			squashPositive(rejection, moveScale)
	}

	window.lastRVOL = rvol
	strength := math.Max(
		ignitionScore,
		math.Max(compressionScore, math.Max(trendScore, exhaustionScore)),
	)

	if math.IsNaN(strength) || math.IsInf(strength, 0) {
		return
	}

	window.cached = IgnitionOutput{
		Value:       strength,
		RVOL:        rvol,
		Precursor:   precursor,
		Compression: compressionScore,
		Ignition:    ignitionScore,
		Trend:       trendScore,
		Exhaustion:  exhaustionScore,
		Strength:    strength,
	}
}

/*
compose returns the held bar scores with the live quote spread applied so the
panel always reflects the current book even between volume-bar closes.
*/
func (window *ignitionWindow) compose(spread float64) IgnitionOutput {
	out := window.cached
	out.Spread = spread

	return out
}

/*
ready reports that at least one volume bar has closed against a formed spread
baseline, so the held event scores reflect real executed evidence.
*/
func (window *ignitionWindow) ready() bool {
	if window.bars <= 0 {
		return false
	}

	_, ok := statistic.MedianOf(window.spreads)

	return ok
}

func (window *ignitionWindow) maturity() float64 {
	return float64(window.bars) / float64(window.bars+1)
}

func geomean(guard bool, values ...float64) float64 {
	if !guard {
		return 0
	}

	score, err := probability.EvidenceGeomean(values...)

	if err != nil {
		return 0
	}

	return score
}

func (window *ignitionWindow) appendPositive(
	values []float64,
	value float64,
) []float64 {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return values
	}

	return window.append(values, value)
}

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
append retains one valid baseline sample without allowing a long-running symbol
to grow calculation cost or memory beyond the configured market history.
*/
func (window *ignitionWindow) append(values []float64, value float64) []float64 {
	if len(values) < window.capacity {
		return append(values, value)
	}

	copy(values, values[1:])
	values[len(values)-1] = value

	return values
}

func squashPositive(value float64, scale float64) float64 {
	if value <= 0 {
		return 0
	}

	resolved := resolveScale(value, scale)

	if resolved <= 0 {
		return 0
	}

	return value / (resolved + value)
}

func inverseSquash(value float64, scale float64) float64 {
	if value < 0 {
		return 0
	}

	if value == 0 {
		return 1
	}

	resolved := resolveScale(value, scale)

	if resolved <= 0 {
		return 0
	}

	return resolved / (resolved + value)
}

func resolveScale(value float64, scale float64) float64 {
	if scale > 0 && !math.IsNaN(scale) && !math.IsInf(scale, 0) {
		return scale
	}

	return magnitudeScale(value)
}

func magnitudeScale(value float64) float64 {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}

	normalized, exponent := math.Frexp(value)

	if normalized == 0 {
		return 0
	}

	return math.Ldexp(1, exponent-1)
}

func ratioScale(values []float64, baseline float64) float64 {
	if baseline <= 0 || len(values) == 0 {
		return 0
	}

	ratios := make([]float64, 0, len(values))

	for _, sample := range values {
		if sample < 0 || math.IsNaN(sample) || math.IsInf(sample, 0) {
			continue
		}

		ratios = append(ratios, sample/baseline)
	}

	median, ok := statistic.MedianOf(ratios)

	if !ok || median <= 0 || math.IsNaN(median) || math.IsInf(median, 0) {
		return 0
	}

	return median
}

func compressionRatioScale(spreads []float64, baseline float64) float64 {
	if baseline <= 0 || len(spreads) == 0 {
		return 0
	}

	compressions := make([]float64, 0, len(spreads))

	for _, spread := range spreads {
		if spread <= 0 || math.IsNaN(spread) || math.IsInf(spread, 0) {
			continue
		}

		compressions = append(compressions, math.Max(0, 1-spread/baseline))
	}

	median, ok := statistic.MedianOf(compressions)

	if !ok || median <= 0 || math.IsNaN(median) || math.IsInf(median, 0) {
		return 0
	}

	return median
}

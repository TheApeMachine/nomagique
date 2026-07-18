package equation

import (
	"math"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/probability"
	"github.com/theapemachine/nomagique/statistic"
)

/*
Ignition scores ticker lift, price precursor, spread compression, and exhaustion.
*/
type Ignition struct {
	windows map[string]*ignitionWindow
}

/*
IgnitionInput is one ticker observation.
*/
type IgnitionInput struct {
	Symbol string
	Volume float64
	Last   float64
	Bid    float64
	Ask    float64
}

/*
IgnitionOutput contains direct ticker ignition scores.
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
	lastVolume   float64
	lastPrice    float64
	lastRVOL     float64
	volumeLift   []float64
	precursors   []float64
	moves        []float64
	spreads      []float64
	initialized  bool
	observations int
}

/*
NewIgnition returns a direct ticker ignition calculator.
*/
func NewIgnition() *Ignition {
	return &Ignition{
		windows: map[string]*ignitionWindow{},
	}
}

/*
Measure scores one ticker observation and reports a confidence maturity
alongside it.
*/
func (ignition *Ignition) Measure(
	input IgnitionInput,
) (IgnitionOutput, bool, float64, error) {
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
	window.observations++
	maturity := float64(window.observations) / float64(window.observations+1)

	if !window.initialized {
		window.lastVolume = input.Volume
		window.lastPrice = input.Last
		window.spreads = append(window.spreads, spread)
		window.initialized = true

		return IgnitionOutput{Spread: spread}, true, maturity, nil
	}

	volumeLift := math.Max(0, input.Volume-window.lastVolume)
	priceMove := math.Log(input.Last / window.lastPrice)
	precursor := math.Max(0, priceMove)
	move := math.Abs(priceMove)
	window.volumeLift = appendNonNegative(window.volumeLift, volumeLift)
	window.precursors = appendNonNegative(window.precursors, precursor)
	window.moves = appendNonNegative(window.moves, move)
	window.spreads = appendPositive(window.spreads, spread)

	output, ready, err := window.measure(volumeLift, priceMove, spread)
	window.lastVolume = input.Volume
	window.lastPrice = input.Last

	return output, ready, maturity, err
}

func (ignition *Ignition) window(symbol string) *ignitionWindow {
	existing, ok := ignition.windows[symbol]

	if ok {
		return existing
	}

	window := &ignitionWindow{}
	ignition.windows[symbol] = window

	return window
}

func (window *ignitionWindow) measure(
	volumeLift float64,
	precursorMove float64,
	spread float64,
) (IgnitionOutput, bool, error) {
	volumeBaseline, volumeReady := statistic.MedianOf(window.volumeLift)
	spreadBaseline, spreadReady := statistic.MedianOf(window.spreads)
	precursorBaseline, precursorReady := statistic.MedianOf(window.precursors)
	moveBaseline, moveReady := statistic.MedianOf(window.moves)

	if !volumeReady || !spreadReady || !precursorReady || !moveReady {
		return IgnitionOutput{}, false, nil
	}

	if volumeBaseline <= 0 || spreadBaseline <= 0 || precursorBaseline <= 0 || moveBaseline <= 0 {
		return IgnitionOutput{}, false, nil
	}

	rvol := volumeLift / volumeBaseline
	precursor := math.Max(0, precursorMove) / precursorBaseline
	compression := math.Max(0, 1-spread/spreadBaseline)
	rejection := math.Max(0, -precursorMove) / moveBaseline

	rvolScale := ratioScale(window.volumeLift, volumeBaseline)
	precursorScale := ratioScale(window.precursors, precursorBaseline)
	moveScale := ratioScale(window.moves, moveBaseline)
	compressionScale := compressionRatioScale(window.spreads, spreadBaseline)
	ignitionScore := 0.0

	if rvol > 0 && precursor > 0 {
		score, err := probability.EvidenceGeomean(rvol, precursor)

		if err != nil {
			return IgnitionOutput{}, false, err
		}

		ignitionScore = score
	}

	trendScore := 0.0

	if rvol > 0 && precursor > 0 {
		score, err := probability.EvidenceGeomean(
			precursor,
			squashPositive(rvol, rvolScale),
			inverseSquash(compression, compressionScale),
		)

		if err != nil {
			return IgnitionOutput{}, false, err
		}

		trendScore = score
	}

	compressionScore := 0.0

	if compression > 0 && rvol > 0 {
		score, err := probability.EvidenceGeomean(
			compression,
			squashPositive(rvol, rvolScale),
			inverseSquash(precursor, precursorScale),
		)

		if err != nil {
			return IgnitionOutput{}, false, err
		}

		compressionScore = score
	}

	rvolDecline := math.Max(0, window.lastRVOL-rvol)
	exhaustionScore := 0.0

	if window.lastRVOL > 0 && rejection > 0 {
		exhaustionScore = (rvolDecline / window.lastRVOL) * squashPositive(rejection, moveScale)
	}

	window.lastRVOL = rvol
	strength := math.Max(
		ignitionScore,
		math.Max(compressionScore, math.Max(trendScore, exhaustionScore)),
	)

	if math.IsNaN(strength) || math.IsInf(strength, 0) {
		return IgnitionOutput{}, false, nil
	}

	return IgnitionOutput{
		Value:       strength,
		RVOL:        rvol,
		Precursor:   precursor,
		Spread:      spread,
		Compression: compressionScore,
		Ignition:    ignitionScore,
		Trend:       trendScore,
		Exhaustion:  exhaustionScore,
		Strength:    strength,
	}, true, nil
}

func appendPositive(values []float64, value float64) []float64 {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return values
	}

	return append(values, value)
}

func appendNonNegative(values []float64, value float64) []float64 {
	if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return values
	}

	return append(values, value)
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

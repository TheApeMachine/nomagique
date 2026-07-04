package algorithm

import (
	"math"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/statistic"
)

const (
	decaySampleHistoryCap = 64
	decaySampleMinHistory = 4
)

/*
DecaySample accumulates book and trade frames into the feature batch Decay expects.
Series lengths are derived from observed update cadence, not fixed constants.
*/
type DecaySample struct {
	windows map[string]*decayWindow
}

type decayWindow struct {
	bids          map[float64]float64
	asks          map[float64]float64
	bidDepthHist  []float64
	askDepthHist  []float64
	densityHist   []float64
	spreadHist    []float64
	pressureHist  []float64
	imbalanceHist []float64
	tradePressure float64
	tradeFrames   int
	lastPrice     float64
}

/*
NewDecaySample returns a book/trade sampler for microstructure decay classification.
*/
func NewDecaySample() *DecaySample {
	return &DecaySample{
		windows: map[string]*decayWindow{},
	}
}

/*
MeasureBook observes one book update and returns decay input when ready.
*/
func (decaySample *DecaySample) MeasureBook(
	input BookflowBookInput,
) (equation.DecayInput, bool, error) {
	if input.Symbol == "" {
		return equation.DecayInput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"decay-sample: symbol required",
			nil,
		))
	}

	window := decaySample.window(input.Symbol)
	decaySample.ingestBook(input, window)

	return decaySample.features(window)
}

/*
MeasureTrade observes one trade update and returns decay input when ready.
*/
func (decaySample *DecaySample) MeasureTrade(
	input BookflowTradeInput,
) (equation.DecayInput, bool, error) {
	if input.Symbol == "" {
		return equation.DecayInput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"decay-sample: symbol required",
			nil,
		))
	}

	if input.Price <= 0 || input.Quantity <= 0 {
		return equation.DecayInput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"decay-sample: trade price and quantity required",
			nil,
		))
	}

	window := decaySample.window(input.Symbol)
	decaySample.ingestTrade(input, window)

	return decaySample.features(window)
}

func (decaySample *DecaySample) window(symbol string) *decayWindow {
	existing, ok := decaySample.windows[symbol]

	if ok {
		return existing
	}

	window := &decayWindow{
		bids: map[float64]float64{},
		asks: map[float64]float64{},
	}

	decaySample.windows[symbol] = window

	return window
}

func (decaySample *DecaySample) ingestBook(input BookflowBookInput, window *decayWindow) {
	decaySample.applyLevels(input.Bids, window.bids)
	decaySample.applyLevels(input.Asks, window.asks)

	mid := bookflowMidPrice(window.bids, window.asks)
	spread := bookflowSpread(window.bids, window.asks)

	if mid <= 0 || spread <= 0 {
		return
	}

	window.lastPrice = mid
	bidDepth := decaySideDepth(window.bids)
	askDepth := decaySideDepth(window.asks)
	density := bidDepth + askDepth
	decayRate := bookflowDecayRate(mid, spread)
	imbalance := bookflowImbalance(window.bids, window.asks, mid, decayRate, false, 0, 0, 0)

	window.bidDepthHist = appendRingFloat(window.bidDepthHist, bidDepth, decaySampleHistoryCap)
	window.askDepthHist = appendRingFloat(window.askDepthHist, askDepth, decaySampleHistoryCap)
	window.densityHist = appendRingFloat(window.densityHist, density, decaySampleHistoryCap)
	window.spreadHist = appendRingFloat(window.spreadHist, spread, decaySampleHistoryCap)
	window.pressureHist = appendRingFloat(window.pressureHist, window.tradePressure, decaySampleHistoryCap)
	window.imbalanceHist = appendRingFloat(window.imbalanceHist, imbalance, decaySampleHistoryCap)
}

func (decaySample *DecaySample) ingestTrade(input BookflowTradeInput, window *decayWindow) {
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

	if window.lastPrice <= 0 {
		window.lastPrice = input.Price
	}
}

func (decaySample *DecaySample) applyLevels(
	levels []BookLevel,
	book map[float64]float64,
) {
	for _, level := range levels {
		if level.Price <= 0 {
			return
		}

		if level.Quantity <= 0 {
			delete(book, level.Price)

			continue
		}

		book[level.Price] = level.Quantity
	}
}

func (decaySample *DecaySample) features(
	window *decayWindow,
) (equation.DecayInput, bool, error) {
	series := [][]float64{
		window.bidDepthHist,
		window.askDepthHist,
		window.densityHist,
		window.spreadHist,
		window.pressureHist,
		window.imbalanceHist,
	}

	minLength := math.MaxInt

	for _, segment := range series {
		if len(segment) < minLength {
			minLength = len(segment)
		}
	}

	if minLength < decaySampleMinHistory {
		return equation.DecayInput{}, false, nil
	}

	_, longWindow, err := statistic.ResolveWindows(window.densityHist, 0, 0)

	if err != nil || minLength < longWindow {
		return equation.DecayInput{}, false, nil
	}

	if window.lastPrice <= 0 {
		return equation.DecayInput{}, false, nil
	}

	return equation.DecayInput{
		LastPrice:  window.lastPrice,
		BidDepths:  append([]float64(nil), window.bidDepthHist...),
		AskDepths:  append([]float64(nil), window.askDepthHist...),
		Densities:  append([]float64(nil), window.densityHist...),
		Spreads:    append([]float64(nil), window.spreadHist...),
		Pressures:  append([]float64(nil), window.pressureHist...),
		Imbalances: append([]float64(nil), window.imbalanceHist...),
	}, true, nil
}

func decaySideDepth(book map[float64]float64) float64 {
	depth := 0.0

	for _, quantity := range book {
		depth += quantity
	}

	return depth
}

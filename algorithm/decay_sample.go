package algorithm

import (
	"math"
	"sync"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/algorithm/book/flow"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/statistic"
	"github.com/theapemachine/nomagique/utils"
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
	mu      sync.RWMutex
}

type decayWindow struct {
	book          *flow.Book
	bidDepthHist  []float64
	askDepthHist  []float64
	densityHist   []float64
	spreadHist    []float64
	pressureHist  []float64
	imbalanceHist []float64
	tradePressure float64
	tradeFrames   int
	lastPrice     float64
	mu            sync.Mutex
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
	input flow.BookInput,
) (equation.DecayInput, bool, error) {
	if input.Symbol == "" {
		return equation.DecayInput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"decay-sample: symbol required",
			nil,
		))
	}

	window := decaySample.window(input.Symbol)
	window.mu.Lock()
	defer window.mu.Unlock()

	if err := decaySample.ingestBook(input, window); err != nil {
		return equation.DecayInput{}, false, err
	}

	return decaySample.features(window)
}

/*
MeasureTrade observes one trade update and returns decay input when ready.
*/
func (decaySample *DecaySample) MeasureTrade(
	input flow.TradeInput,
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
	window.mu.Lock()
	defer window.mu.Unlock()

	decaySample.ingestTrade(input, window)

	return decaySample.features(window)
}

func (decaySample *DecaySample) window(symbol string) *decayWindow {
	decaySample.mu.RLock()
	existing, ok := decaySample.windows[symbol]
	decaySample.mu.RUnlock()

	if ok {
		return existing
	}

	// Create new window under write lock
	decaySample.mu.Lock()
	defer decaySample.mu.Unlock()
	existing = decaySample.windows[symbol]
	if existing != nil {
		return existing
	}

	window := &decayWindow{
		book: flow.NewBook(),
	}
	decaySample.windows[symbol] = window

	return window
}

func (decaySample *DecaySample) ingestBook(
	input flow.BookInput,
	window *decayWindow,
) error {
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

	if mid <= 0 || spread <= 0 {
		return nil
	}

	window.lastPrice = mid
	bidDepth := window.book.SideDepth(flow.SideBid)
	askDepth := window.book.SideDepth(flow.SideAsk)
	density := bidDepth + askDepth
	decayRate := flow.DecayRate(mid, spread)
	imbalance := window.book.Imbalance(mid, decayRate, false, 0, 0, 0)

	window.bidDepthHist = utils.AppendRingFloat(window.bidDepthHist, bidDepth, decaySampleHistoryCap)
	window.askDepthHist = utils.AppendRingFloat(window.askDepthHist, askDepth, decaySampleHistoryCap)
	window.densityHist = utils.AppendRingFloat(window.densityHist, density, decaySampleHistoryCap)
	window.spreadHist = utils.AppendRingFloat(window.spreadHist, spread, decaySampleHistoryCap)
	window.pressureHist = utils.AppendRingFloat(window.pressureHist, window.tradePressure, decaySampleHistoryCap)
	window.imbalanceHist = utils.AppendRingFloat(window.imbalanceHist, imbalance, decaySampleHistoryCap)

	return nil
}

func (decaySample *DecaySample) ingestTrade(input flow.TradeInput, window *decayWindow) {
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

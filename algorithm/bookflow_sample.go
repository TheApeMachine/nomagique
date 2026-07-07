package algorithm

import (
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/statistic"
)

const (
	bookflowSampleHistoryCap = 64
)

/*
BookflowSample accumulates book and trade frames into the feature batch Bookflow expects.
Decay rate and history windows are derived from observed spread and imbalance history.
*/
type BookflowSample struct {
	windows map[string]*bookflowWindow
}

/*
BookLevel is one price/quantity level.
*/
type BookLevel struct {
	Price    float64
	Ticks    int64
	Quantity float64
}

/*
BookflowBookInput is one book update for a symbol.
*/
type BookflowBookInput struct {
	Symbol   string
	TickSize float64
	Bids     []BookLevel
	Asks     []BookLevel
}

/*
BookflowTradeInput is one trade update for a symbol.
*/
type BookflowTradeInput struct {
	Symbol   string
	Price    float64
	Quantity float64
	Side     string
}

type bookflowWindow struct {
	book            *bookflowBook
	weightedHist    []float64
	level1Hist      []float64
	flatHist        []float64
	tradePressure   float64
	tradeFrameCount int
	lastMid         float64
	lastSpread      float64
	touchDepth      float64
	flatOK          float64
	touchCancelBid  float64
	touchCancelAsk  float64
	frameAddBid     float64
	frameAddAsk     float64
}

/*
NewBookflowSample returns a book/trade sampler for depth-flow classification.
*/
func NewBookflowSample() *BookflowSample {
	return &BookflowSample{
		windows: map[string]*bookflowWindow{},
	}
}

/*
MeasureBook observes one book update and returns book-flow input when ready.
*/
func (bookflowSample *BookflowSample) MeasureBook(
	input BookflowBookInput,
) (equation.BookflowInput, bool, error) {
	if input.Symbol == "" {
		return equation.BookflowInput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"bookflow-sample: symbol required",
			nil,
		))
	}

	window := bookflowSample.window(input.Symbol)
	if err := bookflowSample.ingestBook(input, window); err != nil {
		return equation.BookflowInput{}, false, err
	}

	return bookflowSample.features(window)
}

/*
MeasureTrade observes one trade update and returns book-flow input when ready.
*/
func (bookflowSample *BookflowSample) MeasureTrade(
	input BookflowTradeInput,
) (equation.BookflowInput, bool, error) {
	if input.Symbol == "" {
		return equation.BookflowInput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"bookflow-sample: symbol required",
			nil,
		))
	}

	if input.Price <= 0 || input.Quantity <= 0 {
		return equation.BookflowInput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"bookflow-sample: trade price and quantity required",
			nil,
		))
	}

	window := bookflowSample.window(input.Symbol)
	bookflowSample.ingestTrade(input, window)

	return bookflowSample.features(window)
}

func (bookflowSample *BookflowSample) window(symbol string) *bookflowWindow {
	existing, ok := bookflowSample.windows[symbol]

	if ok {
		return existing
	}

	window := &bookflowWindow{
		book: newBookflowBook(),
	}

	bookflowSample.windows[symbol] = window

	return window
}

func (bookflowSample *BookflowSample) ingestBook(
	input BookflowBookInput,
	window *bookflowWindow,
) error {
	window.touchCancelBid = 0
	window.touchCancelAsk = 0
	window.frameAddBid = 0
	window.frameAddAsk = 0

	if err := window.book.Configure(input); err != nil {
		return err
	}

	bidFrame, err := window.book.ApplyLevels(input.Bids, SideBid)

	if err != nil {
		return err
	}

	askFrame, err := window.book.ApplyLevels(input.Asks, SideAsk)

	if err != nil {
		return err
	}

	window.touchCancelBid = bidFrame.touchCancel
	window.touchCancelAsk = askFrame.touchCancel
	window.frameAddBid = bidFrame.frameAdd
	window.frameAddAsk = askFrame.frameAdd
	mid := window.book.Mid()
	spread := window.book.Spread()

	if mid <= 0 || spread <= 0 {
		return nil
	}

	decayRate := bookflowDecayRate(mid, spread)
	touchDepth := window.book.TouchDepth()
	toxicBid := bookflowToxicPenalty(window.touchCancelBid, window.frameAddBid, touchDepth)
	toxicAsk := bookflowToxicPenalty(window.touchCancelAsk, window.frameAddAsk, touchDepth)
	weighted := window.book.Imbalance(mid, decayRate, false, 0, toxicBid, toxicAsk)
	level1 := window.book.Imbalance(mid, decayRate, true, 0, toxicBid, toxicAsk)
	flatDepth, err := window.book.FlatDepth()

	window.lastMid = mid
	window.lastSpread = spread
	window.touchDepth = touchDepth
	window.flatOK = 1
	window.weightedHist = appendRingFloat(window.weightedHist, weighted, bookflowSampleHistoryCap)
	window.level1Hist = appendRingFloat(window.level1Hist, level1, bookflowSampleHistoryCap)

	if err != nil {
		return errnie.Error(errnie.Err(
			errnie.Validation,
			"bookflow-sample: flat depth resolution failed",
			err,
		))
	}

	if flatDepth > 0 {
		window.flatOK = 1
	}

	flat := window.book.Imbalance(mid, decayRate, false, flatDepth, toxicBid, toxicAsk)
	window.flatHist = appendRingFloat(window.flatHist, flat, bookflowSampleHistoryCap)

	return nil
}

func (bookflowSample *BookflowSample) ingestTrade(
	input BookflowTradeInput,
	window *bookflowWindow,
) {
	notional := input.Price * input.Quantity
	signedNotional := notional

	if input.Side == "sell" {
		signedNotional = -notional
	}

	window.tradeFrameCount++
	smoothing := 2.0 / float64(window.tradeFrameCount+1)

	if smoothing > 1 {
		smoothing = 1
	}

	window.tradePressure += smoothing * (signedNotional - window.tradePressure)
}

func (bookflowSample *BookflowSample) features(
	window *bookflowWindow,
) (equation.BookflowInput, bool, error) {
	historyCount := len(window.weightedHist)

	if historyCount == 0 {
		return equation.BookflowInput{
			Mid:           window.lastMid,
			Spread:        window.lastSpread,
			TouchDepth:    window.touchDepth,
			TradePressure: window.tradePressure,
		}, true, nil
	}

	_, longWindow, err := statistic.ResolveWindows(window.weightedHist, 0, 0)

	if err != nil || historyCount < longWindow {
		return equation.BookflowInput{}, false, nil
	}

	weighted := window.weightedHist[len(window.weightedHist)-1]
	level1 := window.level1Hist[len(window.level1Hist)-1]
	flat := window.flatHist[len(window.flatHist)-1]

	return equation.BookflowInput{
		Weighted:        weighted,
		Level1:          level1,
		Flat:            flat,
		FlatOK:          window.flatOK > 0,
		Mid:             window.lastMid,
		Spread:          window.lastSpread,
		TouchDepth:      window.touchDepth,
		TradePressure:   window.tradePressure,
		WeightedHistory: append([]float64(nil), window.weightedHist...),
		Level1History:   append([]float64(nil), window.level1Hist...),
		FlatHistory:     append([]float64(nil), window.flatHist...),
	}, true, nil
}

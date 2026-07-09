package flow

import (
	"sync"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/statistic"
	"github.com/theapemachine/nomagique/utils"
)

const (
	SampleHistoryCap = 64
)

/*
Sample accumulates book and trade frames into the feature batch expects.
Decay rate and history windows are derived from observed spread and imbalance history.
*/
type Sample struct {
	windows map[string]*Window
	mu      sync.RWMutex
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
BookInput is one book update for a symbol.
*/
type BookInput struct {
	Symbol   string
	TickSize float64
	Bids     []BookLevel
	Asks     []BookLevel
}

/*
TradeInput is one trade update for a symbol.
*/
type TradeInput struct {
	Symbol   string
	Price    float64
	Quantity float64
	Side     string
}

type Window struct {
	mu sync.Mutex

	book            *Book
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
NewSample returns a book/trade sampler for depth-flow classification.
*/
func NewSample() *Sample {
	return &Sample{
		windows: map[string]*Window{},
	}
}

/*
MeasureBook observes one book update and returns book-flow input when ready.
*/
func (s *Sample) MeasureBook(
	input BookInput,
) (equation.BookflowInput, bool, error) {
	if input.Symbol == "" {
		return equation.BookflowInput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"-sample: symbol required",
			nil,
		))
	}

	s.mu.Lock()
	window := s.windows[input.Symbol]
	if window == nil {
		window = &Window{
			book: NewBook(),
		}
		s.windows[input.Symbol] = window
	}
	s.mu.Unlock()

	window.mu.Lock()
	defer window.mu.Unlock()

	if err := s.ingestBook(input, window); err != nil {
		return equation.BookflowInput{}, false, err
	}

	return s.features(window)
}

/*
MeasureTrade observes one trade update and returns book-flow input when ready.
*/
func (s *Sample) MeasureTrade(
	input TradeInput,
) (equation.BookflowInput, bool, error) {
	if input.Symbol == "" {
		return equation.BookflowInput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"-sample: symbol required",
			nil,
		))
	}

	if input.Price <= 0 || input.Quantity <= 0 {
		return equation.BookflowInput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"-sample: trade price and quantity required",
			nil,
		))
	}

	s.mu.Lock()
	window := s.windows[input.Symbol]
	if window == nil {
		window = &Window{
			book: NewBook(),
		}
		s.windows[input.Symbol] = window
	}
	s.mu.Unlock()

	window.mu.Lock()
	defer window.mu.Unlock()

	s.ingestTrade(input, window)

	return s.features(window)
}

func (s *Sample) ingestBook(
	input BookInput,
	window *Window,
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

	decayRate := DecayRate(mid, spread)
	touchDepth := window.book.TouchDepth()
	toxicBid := ToxicPenalty(window.touchCancelBid, window.frameAddBid, touchDepth)
	toxicAsk := ToxicPenalty(window.touchCancelAsk, window.frameAddAsk, touchDepth)
	weighted := window.book.Imbalance(mid, decayRate, false, 0, toxicBid, toxicAsk)
	level1 := window.book.Imbalance(mid, decayRate, true, 0, toxicBid, toxicAsk)
	flatDepth, err := window.book.FlatDepth()

	window.lastMid = mid
	window.lastSpread = spread
	window.touchDepth = touchDepth
	window.flatOK = 1
	window.weightedHist = utils.AppendRingFloat(window.weightedHist, weighted, SampleHistoryCap)
	window.level1Hist = utils.AppendRingFloat(window.level1Hist, level1, SampleHistoryCap)

	if err != nil {
		return errnie.Error(errnie.Err(
			errnie.Validation,
			"-sample: flat depth resolution failed",
			err,
		))
	}

	if flatDepth > 0 {
		window.flatOK = 1
	}

	flat := window.book.Imbalance(mid, decayRate, false, flatDepth, toxicBid, toxicAsk)
	window.flatHist = utils.AppendRingFloat(window.flatHist, flat, SampleHistoryCap)

	return nil
}

func (s *Sample) ingestTrade(
	input TradeInput,
	window *Window,
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

func (s *Sample) features(
	window *Window,
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

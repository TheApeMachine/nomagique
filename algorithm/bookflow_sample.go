package algorithm

import (
	"math"

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
	Quantity float64
}

/*
BookflowBookInput is one book update for a symbol.
*/
type BookflowBookInput struct {
	Symbol string
	Bids   []BookLevel
	Asks   []BookLevel
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
	bids            map[float64]float64
	asks            map[float64]float64
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
	bookflowSample.ingestBook(input, window)

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
		bids: map[float64]float64{},
		asks: map[float64]float64{},
	}

	bookflowSample.windows[symbol] = window

	return window
}

func (bookflowSample *BookflowSample) ingestBook(input BookflowBookInput, window *bookflowWindow) {
	window.touchCancelBid = 0
	window.touchCancelAsk = 0
	window.frameAddBid = 0
	window.frameAddAsk = 0

	bookflowSample.applyLevels(input.Bids, window.bids, window, SideBid)
	bookflowSample.applyLevels(input.Asks, window.asks, window, SideAsk)

	mid := bookflowMidPrice(window.bids, window.asks)
	spread := bookflowSpread(window.bids, window.asks)

	if mid <= 0 || spread <= 0 {
		return
	}

	decayRate := bookflowDecayRate(mid, spread)
	touchDepth := bookflowTouchDepth(window.bids, window.asks)
	toxicBid := bookflowToxicPenalty(window.touchCancelBid, window.frameAddBid, touchDepth)
	toxicAsk := bookflowToxicPenalty(window.touchCancelAsk, window.frameAddAsk, touchDepth)
	weighted := bookflowImbalance(window.bids, window.asks, mid, decayRate, false, 0, toxicBid, toxicAsk)
	level1 := bookflowImbalance(window.bids, window.asks, mid, decayRate, true, 0, toxicBid, toxicAsk)
	flatDepth, flatDepthErr := bookflowFlatDepth(window.bids, window.asks)

	window.lastMid = mid
	window.lastSpread = spread
	window.touchDepth = touchDepth
	window.flatOK = 1
	window.weightedHist = appendRingFloat(window.weightedHist, weighted, bookflowSampleHistoryCap)
	window.level1Hist = appendRingFloat(window.level1Hist, level1, bookflowSampleHistoryCap)

	if flatDepthErr != nil {
		errnie.Error(errnie.Err(
			errnie.Validation,
			"bookflow-sample: flat depth resolution failed",
			flatDepthErr,
		))

		return
	}

	if flatDepth > 0 {
		window.flatOK = 1
	}

	flat := bookflowImbalance(window.bids, window.asks, mid, decayRate, false, flatDepth, toxicBid, toxicAsk)
	window.flatHist = appendRingFloat(window.flatHist, flat, bookflowSampleHistoryCap)
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

func (bookflowSample *BookflowSample) applyLevels(
	levels []BookLevel,
	book map[float64]float64,
	window *bookflowWindow,
	side byte,
) {
	for _, level := range levels {
		if level.Price <= 0 {
			return
		}

		previousQty := book[level.Price]

		if level.Quantity <= 0 {
			delete(book, level.Price)

			if previousQty > 0 && bookflowSample.isTouchPrice(side, level.Price, book) {
				if side == SideBid {
					window.touchCancelBid += previousQty
				}

				if side == SideAsk {
					window.touchCancelAsk += previousQty
				}
			}

			continue
		}

		delta := level.Quantity - previousQty
		book[level.Price] = level.Quantity

		if delta <= 0 {
			removed := -delta

			if bookflowSample.isTouchPrice(side, level.Price, book) {
				if side == SideBid {
					window.touchCancelBid += removed
				}

				if side == SideAsk {
					window.touchCancelAsk += removed
				}
			}

			continue
		}

		if side == SideBid {
			window.frameAddBid += delta
		}

		if side == SideAsk {
			window.frameAddAsk += delta
		}
	}
}

func (bookflowSample *BookflowSample) isTouchPrice(side byte, price float64, book map[float64]float64) bool {
	if side == SideBid {
		return price == bookflowBestBid(book)
	}

	return price == bookflowBestAsk(book)
}

func bookflowToxicPenalty(touchCancel, frameAdd, touchDepth float64) float64 {
	if touchCancel <= 0 || frameAdd <= 0 {
		return 0
	}

	churn := touchCancel / frameAdd

	if touchDepth <= 0 {
		return math.Min(1, churn)
	}

	return math.Min(1, churn*(touchCancel/touchDepth))
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

func bookflowMidPrice(bids, asks map[float64]float64) float64 {
	bestBid := bookflowBestBid(bids)
	bestAsk := bookflowBestAsk(asks)

	if bestBid <= 0 || bestAsk <= 0 {
		return 0
	}

	return (bestBid + bestAsk) / 2
}

func bookflowSpread(bids, asks map[float64]float64) float64 {
	bestBid := bookflowBestBid(bids)
	bestAsk := bookflowBestAsk(asks)

	if bestBid <= 0 || bestAsk <= 0 || bestAsk <= bestBid {
		return 0
	}

	return bestAsk - bestBid
}

func bookflowBestBid(bids map[float64]float64) float64 {
	best := 0.0

	for price := range bids {
		if price > best {
			best = price
		}
	}

	return best
}

func bookflowBestAsk(asks map[float64]float64) float64 {
	best := math.Inf(1)

	for price := range asks {
		if price < best {
			best = price
		}
	}

	if math.IsInf(best, 1) {
		return 0
	}

	return best
}

func bookflowTouchDepth(bids, asks map[float64]float64) float64 {
	bestBid := bookflowBestBid(bids)
	bestAsk := bookflowBestAsk(asks)

	depth := 0.0

	if bestBid > 0 {
		depth += bids[bestBid]
	}

	if bestAsk > 0 {
		depth += asks[bestAsk]
	}

	return depth
}

func bookflowDecayRate(mid, spread float64) float64 {
	if mid <= 0 {
		return 1
	}

	relativeSpread := spread / mid

	if relativeSpread <= 0 {
		return 1
	}

	return 1 / relativeSpread
}

func bookflowFlatDepth(bids, asks map[float64]float64) (int, error) {
	levelCount := len(bids) + len(asks)

	if levelCount < 2 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"bookflow-sample: flat depth needs at least two levels",
			nil,
		))
	}

	_, longWindow, err := statistic.ResolveWindows(make([]float64, levelCount), 0, 0)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"bookflow-sample: flat depth window resolution failed",
			err,
		))
	}

	flatDepth := int(math.Ceil(math.Sqrt(float64(levelCount))))

	if flatDepth < 2 {
		flatDepth = 2
	}

	if flatDepth > longWindow {
		flatDepth = longWindow
	}

	return flatDepth, nil
}

func bookflowImbalance(
	bids, asks map[float64]float64,
	mid, decayRate float64,
	touchOnly bool,
	flatDepth int,
	toxicBid, toxicAsk float64,
) float64 {
	bidWeight := bookflowSideWeight(bids, mid, decayRate, touchOnly, flatDepth, SideBid)
	askWeight := bookflowSideWeight(asks, mid, decayRate, touchOnly, flatDepth, SideAsk)

	if toxicBid > 0 {
		bidWeight *= 1 - toxicBid
	}

	if toxicAsk > 0 {
		askWeight *= 1 - toxicAsk
	}

	total := bidWeight + askWeight

	if total <= 0 {
		return 0
	}

	return (bidWeight - askWeight) / total
}

func bookflowSideWeight(
	book map[float64]float64,
	mid, decayRate float64,
	touchOnly bool,
	flatDepth int,
	side byte,
) float64 {
	if touchOnly {
		return bookflowTouchQty(book, side)
	}

	weight := 0.0
	remaining := flatDepth
	prices := bookflowSortedPrices(book, mid)

	for _, price := range prices {
		if flatDepth > 0 {
			if remaining <= 0 {
				break
			}

			remaining--
		}

		quantity := book[price]
		distance := math.Abs(price-mid) / mid
		kernel := math.Exp(-decayRate * distance)
		weight += quantity * kernel
	}

	return weight
}

func bookflowTouchQty(book map[float64]float64, side byte) float64 {
	if len(book) == 0 {
		return 0
	}

	if side == SideBid {
		return book[bookflowBestBid(book)]
	}

	return book[bookflowBestAsk(book)]
}

func bookflowSortedPrices(book map[float64]float64, mid float64) []float64 {
	prices := make([]float64, 0, len(book))

	for price := range book {
		prices = append(prices, price)
	}

	for left := 1; left < len(prices); left++ {
		cursor := prices[left]

		for index := left - 1; index >= 0; index-- {
			leftDistance := math.Abs(prices[index] - mid)
			cursorDistance := math.Abs(cursor - mid)

			if leftDistance <= cursorDistance {
				break
			}

			prices[index+1] = prices[index]
			prices[index] = cursor
		}
	}

	return prices
}

package algorithm

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/statistic"
)

const (
	bookflowSampleHistoryCap = 64
	bookflowSampleMinHistory = 4
)

/*
BookflowSample accumulates book and trade frames into the feature batch Bookflow expects.
Decay rate and history windows are derived from observed spread and imbalance history.
*/
type BookflowSample struct {
	artifact *datura.Artifact
	windows  map[string]*bookflowWindow
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
NewBookflowSample returns a book/trade encoder for depth-flow classification.
*/
func NewBookflowSample(artifact *datura.Artifact) *BookflowSample {
	return &BookflowSample{
		artifact: artifact,
		windows:  map[string]*bookflowWindow{},
	}
}

func (bookflowSample *BookflowSample) Write(payload []byte) (int, error) {
	bookflowSample.artifact.WithPayload(payload)
	return len(payload), nil
}

func (bookflowSample *BookflowSample) Read(payload []byte) (int, error) {
	state := datura.Acquire("bookflow-sample-state", datura.APPJSON)

	if _, err := state.Write(bookflowSample.artifact.DecryptPayload()); err != nil {
		state.Release()

		return 0, err
	}

	defer state.Release()

	channel := datura.Peek[string](state, "channel")
	symbol := datura.Peek[string](state, "data", 0, "symbol")

	if symbol == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"bookflow-sample: symbol required",
			nil,
		))
	}

	window := bookflowSample.window(symbol)

	switch channel {
	case "book":
		bookflowSample.ingestBook(state, window)
	case "trade":
		bookflowSample.ingestTrade(state, window)
	}

	features := bookflowSample.features(window)

	if len(features) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"bookflow-sample: insufficient history",
			nil,
		))
	}

	state.WithScope(symbol)
	state.Merge("features", features)
	state.Merge("root", "features")
	state.Merge("inputs", equation.BookflowInputKeys)

	return state.Read(payload)
}

func (bookflowSample *BookflowSample) Close() error {
	return nil
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

func (bookflowSample *BookflowSample) ingestBook(state *datura.Artifact, window *bookflowWindow) {
	window.touchCancelBid = 0
	window.touchCancelAsk = 0
	window.frameAddBid = 0
	window.frameAddAsk = 0

	bookflowSample.applyLevels(state, "bids", window.bids, window, SideBid)
	bookflowSample.applyLevels(state, "asks", window.asks, window, SideAsk)

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
	flatDepth := bookflowFlatDepth(window.bids, window.asks)
	flat := bookflowImbalance(window.bids, window.asks, mid, decayRate, false, flatDepth, toxicBid, toxicAsk)

	window.lastMid = mid
	window.lastSpread = spread
	window.touchDepth = touchDepth
	window.flatOK = 1

	if flatDepth > 0 {
		window.flatOK = 1
	}

	window.weightedHist = appendRingFloat(window.weightedHist, weighted, bookflowSampleHistoryCap)
	window.level1Hist = appendRingFloat(window.level1Hist, level1, bookflowSampleHistoryCap)
	window.flatHist = appendRingFloat(window.flatHist, flat, bookflowSampleHistoryCap)
}

func (bookflowSample *BookflowSample) ingestTrade(state *datura.Artifact, window *bookflowWindow) {
	price := datura.Peek[float64](state, "data", 0, "price")
	quantity := datura.Peek[float64](state, "data", 0, "qty")
	side := datura.Peek[string](state, "data", 0, "side")

	if price <= 0 || quantity <= 0 {
		return
	}

	notional := price * quantity
	signedNotional := notional

	if side == "sell" {
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
	state *datura.Artifact,
	sideKey string,
	book map[float64]float64,
	window *bookflowWindow,
	side byte,
) {
	for index := 0; ; index++ {
		price := datura.Peek[float64](state, "data", 0, sideKey, index, "price")
		quantity := datura.Peek[float64](state, "data", 0, sideKey, index, "qty")

		if price <= 0 {
			break
		}

		previousQty := book[price]

		if quantity <= 0 {
			delete(book, price)

			if previousQty > 0 && bookflowSample.isTouchPrice(side, price, book) {
				if side == SideBid {
					window.touchCancelBid += previousQty
				}

				if side == SideAsk {
					window.touchCancelAsk += previousQty
				}
			}

			continue
		}

		delta := quantity - previousQty
		book[price] = quantity

		if delta <= 0 {
			removed := -delta

			if bookflowSample.isTouchPrice(side, price, book) {
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

func (bookflowSample *BookflowSample) features(window *bookflowWindow) []float64 {
	historyCount := len(window.weightedHist)

	if historyCount < bookflowSampleMinHistory {
		return nil
	}

	_, longWindow, err := statistic.RollingWindows(
		window.weightedHist, 0, 0,
	)

	if err != nil || historyCount < longWindow {
		return nil
	}

	weightedCount := len(window.weightedHist)
	level1Count := len(window.level1Hist)
	flatCount := len(window.flatHist)

	features := make([]float64, 0, 11+weightedCount+level1Count+flatCount)
	weighted := window.weightedHist[len(window.weightedHist)-1]
	level1 := window.level1Hist[len(window.level1Hist)-1]
	flat := window.flatHist[len(window.flatHist)-1]

	features = append(features,
		weighted,
		level1,
		flat,
		window.flatOK,
		window.lastMid,
		window.lastSpread,
		window.touchDepth,
		window.tradePressure,
		float64(weightedCount),
		float64(level1Count),
		float64(flatCount),
	)
	features = append(features, window.weightedHist...)
	features = append(features, window.level1Hist...)
	features = append(features, window.flatHist...)

	return features
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

func bookflowFlatDepth(bids, asks map[float64]float64) int {
	levelCount := len(bids) + len(asks)

	if levelCount < 2 {
		return 0
	}

	_, longWindow, err := statistic.RollingWindows(make([]float64, levelCount), 0, 0)

	if err != nil {
		return 2
	}
	flatDepth := int(math.Ceil(math.Sqrt(float64(levelCount))))

	if flatDepth < 2 {
		flatDepth = 2
	}

	if flatDepth > longWindow {
		flatDepth = longWindow
	}

	return flatDepth
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

package algorithm

import (
	"math"

	"github.com/theapemachine/datura"
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
	artifact *datura.Artifact
	windows  map[string]*decayWindow
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
NewDecaySample returns a book/trade encoder for microstructure decay classification.
*/
func NewDecaySample(artifact *datura.Artifact) *DecaySample {
	return &DecaySample{
		artifact: artifact,
		windows:  map[string]*decayWindow{},
	}
}

func (decaySample *DecaySample) Write(payload []byte) (int, error) {
	decaySample.artifact.WithPayload(payload)
	return len(payload), nil
}

func (decaySample *DecaySample) Read(payload []byte) (int, error) {
	state := datura.Acquire("decay-sample-state", datura.APPJSON)

	if _, err := state.Write(decaySample.artifact.DecryptPayload()); err != nil {
		state.Release()

		return 0, err
	}

	defer state.Release()

	channel := datura.Peek[string](state, "channel")
	symbol := datura.Peek[string](state, "data", 0, "symbol")

	if symbol == "" {
		return state.Read(payload)
	}

	window := decaySample.window(symbol)

	switch channel {
	case "book":
		decaySample.ingestBook(state, window)
	case "trade":
		decaySample.ingestTrade(state, window)
	}

	features := decaySample.features(window)

	if len(features) == 0 {
		return state.Read(payload)
	}

	state.WithScope(symbol)
	state.Merge("features", features)
	state.Merge("root", "features")
	state.Merge("inputs", equation.DecayInputKeys)

	return state.Read(payload)
}

func (decaySample *DecaySample) Close() error {
	return nil
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

func (decaySample *DecaySample) ingestBook(state *datura.Artifact, window *decayWindow) {
	decaySample.applyLevels(state, "bids", window.bids)
	decaySample.applyLevels(state, "asks", window.asks)

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

func (decaySample *DecaySample) ingestTrade(state *datura.Artifact, window *decayWindow) {
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

	window.tradeFrames++
	smoothing := 2.0 / float64(window.tradeFrames+1)

	if smoothing > 1 {
		smoothing = 1
	}

	window.tradePressure += smoothing * (signedNotional - window.tradePressure)

	if window.lastPrice <= 0 {
		window.lastPrice = price
	}
}

func (decaySample *DecaySample) applyLevels(
	state *datura.Artifact,
	sideKey string,
	book map[float64]float64,
) {
	for index := 0; ; index++ {
		price := datura.Peek[float64](state, "data", 0, sideKey, index, "price")
		quantity := datura.Peek[float64](state, "data", 0, sideKey, index, "qty")

		if price <= 0 {
			break
		}

		if quantity <= 0 {
			delete(book, price)

			continue
		}

		book[price] = quantity
	}
}

func (decaySample *DecaySample) features(window *decayWindow) []float64 {
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
		return nil
	}

	_, longWindow, err := statistic.RollingWindows(window.densityHist, 0, 0)

	if err != nil || minLength < longWindow {
		return nil
	}

	if window.lastPrice <= 0 {
		return nil
	}

	payload := []float64{window.lastPrice}

	for _, segment := range series {
		payload = append(payload, float64(len(segment)))
	}

	for _, segment := range series {
		payload = append(payload, segment...)
	}

	return payload
}

func decaySideDepth(book map[float64]float64) float64 {
	depth := 0.0

	for _, quantity := range book {
		depth += quantity
	}

	return depth
}

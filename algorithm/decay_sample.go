package algorithm

import (
	"io"
	"math"

	"github.com/theapemachine/datura"
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
	artifact     *datura.Artifact
	pendingFrame bool
	output       []byte
	windows      map[string]*decayWindow
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
	if len(payload) == 0 {
		decaySample.pendingFrame = false
		decaySample.output = nil

		return 0, nil
	}

	decaySample.artifact.WithPayload(payload)
	decaySample.pendingFrame = true
	decaySample.output = nil

	return len(payload), nil
}

func (decaySample *DecaySample) Read(payload []byte) (int, error) {
	if len(decaySample.output) > 0 {
		return decaySample.readOutput(payload)
	}

	if !decaySample.pendingFrame {
		return 0, io.EOF
	}

	state := datura.Acquire("decay-sample-state", datura.APPJSON)
	frame := decaySample.artifact.DecryptPayload()

	if len(frame) == 0 {
		state.Release()
		decaySample.pendingFrame = false

		return 0, io.EOF
	}

	if _, err := state.Unpack(frame); err != nil {
		state.Release()
		decaySample.pendingFrame = false

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"decay-sample: state write failed",
			err,
		))
	}

	defer state.Release()

	channel := datura.Peek[string](state, "channel")
	symbol := datura.Peek[string](state, "data", 0, "symbol")
	row := false

	if symbol == "" {
		symbol = datura.Peek[string](state, "symbol")
		row = symbol != ""
	}

	if channel == "" && row {
		if datura.Peek[float64](state, "bids", 0, "price") > 0 ||
			datura.Peek[float64](state, "asks", 0, "price") > 0 {
			channel = "book"
		}

		if datura.Peek[float64](state, "price") > 0 &&
			datura.Peek[float64](state, "qty") > 0 {
			channel = "trade"
		}
	}

	if symbol == "" {
		decaySample.pendingFrame = false

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"decay-sample: symbol required",
			nil,
		))
	}

	window := decaySample.window(symbol)

	switch channel {
	case "book":
		decaySample.ingestBook(state, window, row)
	case "trade":
		decaySample.ingestTrade(state, window, row)
	}

	features := decaySample.features(window)

	if len(features) == 0 {
		decaySample.pendingFrame = false

		return 0, io.EOF
	}

	state.WithScope(symbol)
	state.Merge("features", features)
	state.MergeOutput("ready", true)
	state.Poke("features", "root")
	state.Poke(equation.DecayInputKeys, "inputs")

	decaySample.output = state.Pack()

	return decaySample.readOutput(payload)
}

func (decaySample *DecaySample) readOutput(payload []byte) (int, error) {
	n := copy(payload, decaySample.output)

	if n < len(decaySample.output) {
		return n, io.ErrShortBuffer
	}

	decaySample.output = nil
	decaySample.pendingFrame = false

	return n, io.EOF
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

func (decaySample *DecaySample) ingestBook(
	state *datura.Artifact,
	window *decayWindow,
	row bool,
) {
	decaySample.applyLevels(state, "bids", window.bids, row)
	decaySample.applyLevels(state, "asks", window.asks, row)

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

func (decaySample *DecaySample) ingestTrade(
	state *datura.Artifact,
	window *decayWindow,
	row bool,
) {
	price := datura.Peek[float64](state, "data", 0, "price")
	quantity := datura.Peek[float64](state, "data", 0, "qty")
	side := datura.Peek[string](state, "data", 0, "side")

	if row {
		price = datura.Peek[float64](state, "price")
		quantity = datura.Peek[float64](state, "qty")
		side = datura.Peek[string](state, "side")
	}

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
	row bool,
) {
	for index := 0; ; index++ {
		price := datura.Peek[float64](state, "data", 0, sideKey, index, "price")
		quantity := datura.Peek[float64](state, "data", 0, sideKey, index, "qty")

		if row {
			price = datura.Peek[float64](state, sideKey, index, "price")
			quantity = datura.Peek[float64](state, sideKey, index, "qty")
		}

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

	_, longWindow, err := statistic.ResolveWindows(window.densityHist, 0, 0)

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

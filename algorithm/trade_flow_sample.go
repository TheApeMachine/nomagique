package algorithm

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/statistic"
)

const (
	flowSampleHeader     = 5
	flowSampleMinTrades  = 2
	flowSampleHistoryCap = 128
)

/*
TradeFlowSample accumulates signed trade notionals into the feature batch Flow expects.
Window sizing and gross floors are derived from observed notionals, not fixed constants.
*/
type TradeFlowSample struct {
	artifact *datura.Artifact
	windows  map[string]*tradeFlowWindow
}

type tradeFlowWindow struct {
	ticks []tradeFlowTick
}

type tradeFlowTick struct {
	buy      bool
	notional float64
	price    float64
}

/*
NewTradeFlowSample returns a trade encoder for CVD flow classification.
*/
func NewTradeFlowSample(artifact *datura.Artifact) *TradeFlowSample {
	return &TradeFlowSample{
		artifact: artifact,
		windows:  map[string]*tradeFlowWindow{},
	}
}

func (tradeFlowSample *TradeFlowSample) Write(payload []byte) (int, error) {
	tradeFlowSample.artifact.WithPayload(payload)
	return len(payload), nil
}

func (tradeFlowSample *TradeFlowSample) Read(payload []byte) (int, error) {
	state := datura.Acquire("trade-flow-sample-state", datura.APPJSON)

	if _, err := state.Write(tradeFlowSample.artifact.DecryptPayload()); err != nil {
		state.Release()

		return 0, err
	}

	defer state.Release()

	channel := datura.Peek[string](state, "channel")

	if channel != "trade" {
		return state.Read(payload)
	}

	symbol := datura.Peek[string](state, "data", 0, "symbol")
	price := datura.Peek[float64](state, "data", 0, "price")
	quantity := datura.Peek[float64](state, "data", 0, "qty")
	side := datura.Peek[string](state, "data", 0, "side")

	if symbol == "" || price <= 0 || quantity <= 0 {
		return state.Read(payload)
	}

	window := tradeFlowSample.window(symbol)
	notional := price * quantity
	window.ticks = append(window.ticks, tradeFlowTick{
		buy:      side == "buy",
		notional: notional,
		price:    price,
	})

	if len(window.ticks) > flowSampleHistoryCap {
		window.ticks = window.ticks[len(window.ticks)-flowSampleHistoryCap:]
	}

	features := tradeFlowSample.features(window)

	if len(features) == 0 {
		return state.Read(payload)
	}

	state.WithScope(symbol)
	state.Merge("features", features)
	state.Merge("root", "features")
	state.Merge("inputs", equation.FlowInputKeys)

	return state.Read(payload)
}

func (tradeFlowSample *TradeFlowSample) Close() error {
	return nil
}

func (tradeFlowSample *TradeFlowSample) window(symbol string) *tradeFlowWindow {
	existing, ok := tradeFlowSample.windows[symbol]

	if ok {
		return existing
	}

	window := &tradeFlowWindow{}
	tradeFlowSample.windows[symbol] = window

	return window
}

func (tradeFlowSample *TradeFlowSample) features(window *tradeFlowWindow) []float64 {
	tradeCount := len(window.ticks)

	if tradeCount < flowSampleMinTrades {
		return nil
	}

	notionals := make([]float64, tradeCount)

	for index, tick := range window.ticks {
		notionals[index] = tick.notional
	}

	_, longWindow, err := statistic.RollingWindows(notionals, 0, 0)

	if err != nil || tradeCount < longWindow {
		return nil
	}

	active := window.ticks[len(window.ticks)-longWindow:]

	buyNotional := 0.0
	sellNotional := 0.0
	prices := make([]float64, 0, len(active))
	activeNotionals := make([]float64, 0, len(active))

	for _, tick := range active {
		if tick.buy {
			buyNotional += tick.notional
		}

		if !tick.buy {
			sellNotional += tick.notional
		}

		prices = append(prices, tick.price)
		activeNotionals = append(activeNotionals, tick.notional)
	}

	grossFloor := medianPositive(activeNotionals)
	medianNotional := grossFloor

	if medianNotional <= 0 {
		return nil
	}

	features := make([]float64, 0, flowSampleHeader+len(prices))
	features = append(features,
		buyNotional,
		sellNotional,
		float64(len(prices)),
		grossFloor,
		medianNotional,
	)
	features = append(features, prices...)

	return features
}

func medianPositive(values []float64) float64 {
	positive := make([]float64, 0, len(values))

	for _, value := range values {
		if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}

		positive = append(positive, value)
	}

	if len(positive) == 0 {
		return 0
	}

	median, ok := statistic.MedianOf(positive)

	if !ok {
		return 0
	}

	return median
}

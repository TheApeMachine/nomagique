package algorithm

import (
	"fmt"
	"math"
	"sync"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/statistic"
)

const (
	flowSampleHistoryCap = 128
)

/*
TradeFlowSample accumulates signed trade notionals into the feature batch Flow expects.
Window sizing and gross floors are derived from observed notionals, not fixed constants.
*/
type TradeFlowSample struct {
	windows *sync.Map
}

/*
TradeFlowInput is one executed trade observation.
*/
type TradeFlowInput struct {
	Symbol   string
	Price    float64
	Quantity float64
	Side     string
}

type tradeFlowWindow struct {
	ticks        []tradeFlowTick
	observations int
}

type tradeFlowTick struct {
	buy      bool
	notional float64
	price    float64
}

/*
NewTradeFlowSample returns a trade sampler for CVD flow classification.
*/
func NewTradeFlowSample() *TradeFlowSample {
	return &TradeFlowSample{
		windows: &sync.Map{},
	}
}

/*
Measure observes one trade and returns flow input, whether the window is
ready to score, and a confidence maturity for that reading.
*/
func (tradeFlowSample *TradeFlowSample) Measure(
	input TradeFlowInput,
) (equation.FlowInput, bool, float64, error) {
	if input.Symbol == "" || input.Price <= 0 || input.Quantity <= 0 {
		return equation.FlowInput{}, false, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"trade-flow-sample: trade frame requires symbol, price, and qty",
			nil,
		))
	}

	if input.Side != "buy" && input.Side != "sell" {
		return equation.FlowInput{}, false, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			fmt.Sprintf("trade-flow-sample: unknown side %q", input.Side),
			nil,
		))
	}

	window := tradeFlowSample.window(input.Symbol)
	notional := input.Price * input.Quantity
	window.ticks = append(window.ticks, tradeFlowTick{
		buy:      input.Side == "buy",
		notional: notional,
		price:    input.Price,
	})
	window.observations++

	if len(window.ticks) > flowSampleHistoryCap {
		window.ticks = window.ticks[len(window.ticks)-flowSampleHistoryCap:]
	}

	features, ok := tradeFlowSample.features(window)
	maturity := float64(window.observations) / float64(window.observations+1)

	if !ok {
		return equation.FlowInput{}, false, maturity, nil
	}

	return features, true, maturity, nil
}

func (tradeFlowSample *TradeFlowSample) window(symbol string) *tradeFlowWindow {
	existing, ok := tradeFlowSample.windows.Load(symbol)

	if ok {
		return existing.(*tradeFlowWindow)
	}

	window := &tradeFlowWindow{}
	tradeFlowSample.windows.Store(symbol, window)

	return window
}

func (tradeFlowSample *TradeFlowSample) features(
	window *tradeFlowWindow,
) (equation.FlowInput, bool) {
	tradeCount := len(window.ticks)

	if tradeCount == 0 {
		return equation.FlowInput{}, false
	}

	notionals := make([]float64, tradeCount)

	for index, tick := range window.ticks {
		notionals[index] = tick.notional
	}

	// ResolveWindowSet always clamps LongWindow to at most tradeCount (the
	// adaptive window can never exceed observed history), so a count
	// shortfall here is structurally impossible — only a resolution error
	// (e.g. non-finite notionals) is a real reason to withhold a reading.
	windows, err := statistic.ResolveWindowSet(notionals, statistic.WindowsConfig{})

	if err != nil {
		return equation.FlowInput{}, false
	}

	activeWindow := windows.ShortWindow
	responseWindow := windows.ReturnLag + 1

	if responseWindow > activeWindow {
		activeWindow = responseWindow
	}

	if activeWindow > windows.LongWindow {
		activeWindow = windows.LongWindow
	}

	active := window.ticks[len(window.ticks)-activeWindow:]
	baseline := window.ticks[len(window.ticks)-windows.LongWindow:]

	buyNotional := 0.0
	sellNotional := 0.0
	prices := make([]float64, 0, len(active))
	activeNotionals := make([]float64, 0, len(active))
	baselineNotionals := make([]float64, 0, len(baseline))

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

	for _, tick := range baseline {
		baselineNotionals = append(baselineNotionals, tick.notional)
	}

	// medianPositive(baselineNotionals) is structurally guaranteed positive:
	// Measure only ever appends strictly positive notionals (Price and
	// Quantity are validated > 0), and baseline always includes at least
	// the tick just appended.
	medianNotional := medianPositive(baselineNotionals)
	grossFloor := medianNotional * float64(len(activeNotionals))

	return equation.FlowInput{
		BuyNotional:    buyNotional,
		SellNotional:   sellNotional,
		TradeCount:     len(prices),
		GrossFloor:     grossFloor,
		MedianNotional: medianNotional,
		Prices:         prices,
	}, true
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

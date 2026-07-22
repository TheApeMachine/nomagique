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
TradeFlowInput is one executed trade observation. Price values the executed
notional while ResponsePrice tracks the quote midpoint response without
mistaking bid-ask execution bounce for directional movement.
*/
type TradeFlowInput struct {
	Symbol        string
	Price         float64
	ResponsePrice float64
	Quantity      float64
	Side          string
}

/*
tradeFlowWindow retains the bounded per-symbol observations used by the
adaptive active and baseline windows.
*/
type tradeFlowWindow struct {
	ticks        []tradeFlowTick
	observations int
}

/*
tradeFlowTick stores the two economically distinct prices needed by CVD as a
signed notional and an execution-bounce-free response price.
*/
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
	if input.Symbol == "" || input.Price <= 0 || input.ResponsePrice <= 0 ||
		input.Quantity <= 0 || math.IsNaN(input.Price) ||
		math.IsInf(input.Price, 0) || math.IsNaN(input.ResponsePrice) ||
		math.IsInf(input.ResponsePrice, 0) || math.IsNaN(input.Quantity) ||
		math.IsInf(input.Quantity, 0) {
		return equation.FlowInput{}, false, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"trade-flow-sample: trade requires symbol, execution price, response price, and qty",
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

	if math.IsNaN(notional) || math.IsInf(notional, 0) {
		return equation.FlowInput{}, false, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"trade-flow-sample: execution notional must be finite",
			nil,
		))
	}

	window.ticks = append(window.ticks, tradeFlowTick{
		buy:      input.Side == "buy",
		notional: notional,
		price:    input.ResponsePrice,
	})
	window.observations++

	if len(window.ticks) > flowSampleHistoryCap {
		window.ticks = window.ticks[len(window.ticks)-flowSampleHistoryCap:]
	}

	maturity := float64(window.observations) / float64(window.observations+1)
	features, err := tradeFlowSample.features(window)

	if err != nil {
		return equation.FlowInput{}, false, maturity, err
	}

	return features, true, maturity, nil
}

/*
window returns the rolling state owned by one symbol, creating it on the first
valid observation.
*/
func (tradeFlowSample *TradeFlowSample) window(symbol string) *tradeFlowWindow {
	existing, ok := tradeFlowSample.windows.Load(symbol)

	if ok {
		return existing.(*tradeFlowWindow)
	}

	window := &tradeFlowWindow{}
	tradeFlowSample.windows.Store(symbol, window)

	return window
}

/*
features resolves the adaptive active window and its empirical notional range
into the numerical input consumed by Flow.
*/
func (tradeFlowSample *TradeFlowSample) features(
	window *tradeFlowWindow,
) (equation.FlowInput, error) {
	tradeCount := len(window.ticks)
	notionals := make([]float64, tradeCount)

	for index, tick := range window.ticks {
		notionals[index] = tick.notional
	}

	windows, err := statistic.ResolveWindowSet(notionals, statistic.WindowsConfig{})

	if err != nil {
		return equation.FlowInput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"trade-flow-sample: resolve adaptive window",
			err,
		))
	}

	activeWindow := max(windows.ShortWindow, windows.ReturnLag+1)
	activeWindow = min(activeWindow, windows.LongWindow)
	active := window.ticks[len(window.ticks)-activeWindow:]
	baseline := window.ticks[len(window.ticks)-windows.LongWindow:]
	buyNotional := 0.0
	sellNotional := 0.0
	prices := make([]float64, 0, len(active))
	baselineNotionals := make([]float64, 0, len(baseline))

	for _, tick := range active {
		prices = append(prices, tick.price)

		if tick.buy {
			buyNotional += tick.notional
			continue
		}

		sellNotional += tick.notional
	}

	for _, tick := range baseline {
		baselineNotionals = append(baselineNotionals, tick.notional)
	}

	// The lower boundary is the empirical center less its own absolute
	// dispersion. A merely below-median window therefore remains ordinary
	// flow; starvation requires gross flow outside the observed lower range.
	medianNotional, lowerNotional := notionalRange(baselineNotionals)
	grossFloor := lowerNotional * float64(len(active))

	return equation.FlowInput{
		BuyNotional:    buyNotional,
		SellNotional:   sellNotional,
		TradeCount:     len(prices),
		GrossFloor:     grossFloor,
		MedianNotional: medianNotional,
		Prices:         prices,
	}, nil
}

/*
notionalRange returns the empirical center and its lower absolute-dispersion
boundary so flow starvation is scaled by the symbol's own observed tape.
*/
func notionalRange(values []float64) (float64, float64) {
	if len(values) == 0 {
		panic(errnie.Error(errnie.Err(
			errnie.Unknown,
			"trade-flow-sample: notional range requires observations",
			nil,
		)))
	}

	median, ok := statistic.MedianOf(values)

	if !ok {
		panic(errnie.Error(errnie.Err(
			errnie.Unknown,
			"trade-flow-sample: invalid notional center",
			nil,
		)))
	}

	deviations := make([]float64, len(values))

	for index, value := range values {
		deviations[index] = math.Abs(value - median)
	}

	dispersion, ok := statistic.MedianOf(deviations)

	if !ok {
		panic(errnie.Error(errnie.Err(
			errnie.Unknown,
			"trade-flow-sample: invalid notional dispersion",
			nil,
		)))
	}

	return median, median - dispersion
}

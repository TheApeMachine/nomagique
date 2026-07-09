package algorithm

import (
	"sync"
	"time"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/algorithm/book/flow"
)

const excitationSampleHistoryCap = 128
const excitationSampleSideCap = 64

/*
TradeExcitationInput is one trade arrival observation for Hawkes excitation.
*/
type TradeExcitationInput struct {
	Symbol    string
	Side      string
	Timestamp time.Time
	UnixNano  int64
}

/*
ExcitationInput contains the direct float batch Excitation expects.
*/
type ExcitationInput struct {
	Symbol             string
	HorizonSeconds     float64
	FitCooldownSeconds float64
	TouchImbalance     float64
	BuySeconds         []float64
	SellSeconds        []float64
}

/*
TradeExcitationSample accumulates trade arrival times into the batch Excitation expects.
Horizon and fit cooldown are derived from the observed arrival span.
*/
type TradeExcitationSample struct {
	windows *sync.Map
}

type tradeExcitationWindow struct {
	buySeconds     []float64
	sellSeconds    []float64
	touchImbalance float64
}

/*
NewTradeExcitationSample returns a trade arrival sampler for Hawkes excitation.
*/
func NewTradeExcitationSample() *TradeExcitationSample {
	return &TradeExcitationSample{
		windows: &sync.Map{},
	}
}

/*
MeasureBook observes touch imbalance for later trade excitation batches.
*/
func (tradeExcitationSample *TradeExcitationSample) MeasureBook(
	input flow.BookInput,
) error {
	if input.Symbol == "" {
		return errnie.Error(errnie.Err(
			errnie.Validation,
			"trade-excitation-sample: symbol required",
			nil,
		))
	}

	window := tradeExcitationSample.window(input.Symbol)
	window.touchImbalance = bookTouchImbalance(input)

	return nil
}

/*
MeasureTrade observes a trade arrival and returns an excitation batch when ready.
*/
func (tradeExcitationSample *TradeExcitationSample) MeasureTrade(
	input TradeExcitationInput,
) (ExcitationInput, bool, error) {
	if input.Symbol == "" || (input.Side != "buy" && input.Side != "sell") {
		return ExcitationInput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"trade-excitation-sample: trade frame requires symbol and buy/sell side",
			nil,
		))
	}

	arrival := input.ArrivalSecond()

	if arrival <= 0 {
		return ExcitationInput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"trade-excitation-sample: timestamp required",
			nil,
		))
	}

	window := tradeExcitationSample.window(input.Symbol)

	if input.Side == "buy" {
		window.buySeconds = append(window.buySeconds, arrival)
	}

	if input.Side == "sell" {
		window.sellSeconds = append(window.sellSeconds, arrival)
	}

	window.trim()

	return window.features(input.Symbol)
}

/*
ArrivalSecond returns the trade arrival in floating-point unix seconds.
*/
func (input TradeExcitationInput) ArrivalSecond() float64 {
	if !input.Timestamp.IsZero() {
		return float64(input.Timestamp.UnixNano()) / float64(time.Second)
	}

	if input.UnixNano > 0 {
		return float64(input.UnixNano) / float64(time.Second)
	}

	return 0
}

func (tradeExcitationSample *TradeExcitationSample) window(
	symbol string,
) *tradeExcitationWindow {
	existing, ok := tradeExcitationSample.windows.Load(symbol)

	if ok {
		return existing.(*tradeExcitationWindow)
	}

	window := &tradeExcitationWindow{}
	tradeExcitationSample.windows.Store(symbol, window)

	return window
}

func (window *tradeExcitationWindow) trim() {
	if len(window.buySeconds) > excitationSampleSideCap {
		window.buySeconds = window.buySeconds[len(
			window.buySeconds,
		)-excitationSampleSideCap:]
	}

	if len(window.sellSeconds) > excitationSampleSideCap {
		window.sellSeconds = window.sellSeconds[len(
			window.sellSeconds,
		)-excitationSampleSideCap:]
	}

	total := len(window.buySeconds) + len(window.sellSeconds)

	if total <= excitationSampleHistoryCap {
		return
	}

	trim := total - excitationSampleHistoryCap

	for trim > 0 && len(window.buySeconds) > 0 {
		window.buySeconds = window.buySeconds[1:]
		trim--
	}

	for trim > 0 && len(window.sellSeconds) > 0 {
		window.sellSeconds = window.sellSeconds[1:]
		trim--
	}
}

func (window *tradeExcitationWindow) features(
	symbol string,
) (ExcitationInput, bool, error) {
	eventCount := len(window.buySeconds) + len(window.sellSeconds)

	// Emit a measurement from whatever the window holds — a signal must produce
	// a reading on every tick, never sit silent behind a warmup gate. With a
	// single event the "mean" is that event's own value; sample sufficiency is
	// communicated downstream as low confidence, not as an absent measurement.
	// The only hard floor is one event, which bounds() requires to index [0];
	// this is always satisfied because features() runs after the current trade
	// has been appended.
	if eventCount == 0 {
		return ExcitationInput{}, false, nil
	}

	first, last := window.bounds()
	span := time.Duration((last - first) * float64(time.Second))

	if span <= 0 {
		span = time.Second
	}

	return ExcitationInput{
		Symbol:             symbol,
		HorizonSeconds:     last,
		FitCooldownSeconds: DeriveFitCooldown(span).Seconds(),
		TouchImbalance:     window.touchImbalance,
		BuySeconds:         append([]float64(nil), window.buySeconds...),
		SellSeconds:        append([]float64(nil), window.sellSeconds...),
	}, true, nil
}

func (window *tradeExcitationWindow) bounds() (float64, float64) {
	allSeconds := append([]float64(nil), window.buySeconds...)
	allSeconds = append(allSeconds, window.sellSeconds...)
	first := allSeconds[0]
	last := allSeconds[0]

	for _, second := range allSeconds[1:] {
		if second < first {
			first = second
		}

		if second > last {
			last = second
		}
	}

	return first, last
}

func bookTouchImbalance(input flow.BookInput) float64 {
	if len(input.Bids) == 0 || len(input.Asks) == 0 {
		return 0
	}

	bidQty := input.Bids[0].Quantity
	askQty := input.Asks[0].Quantity
	total := bidQty + askQty

	if total <= 0 {
		return 0
	}

	return (bidQty - askQty) / total
}

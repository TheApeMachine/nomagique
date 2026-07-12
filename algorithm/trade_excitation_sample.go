package algorithm

import (
	"sync"
	"time"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/algorithm/book/flow"
	"github.com/theapemachine/nomagique/hawkes"
)

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
ExcitationArrivalInput carries typed arrival time directly into Hawkes fitting.
*/
type ExcitationArrivalInput struct {
	Symbol         string
	Horizon        time.Time
	FitCooldown    time.Duration
	TouchImbalance float64
	Stream         hawkes.ArrivalStream
	buySeconds     []float64
	sellSeconds    []float64
	horizonSeconds float64
}

/*
TradeExcitationSample accumulates trade arrival times into the batch Excitation expects.
Horizon and fit cooldown are derived from the observed arrival span.
*/
type TradeExcitationSample struct {
	windows *sync.Map
}

type tradeExcitationWindow struct {
	arrivals       *hawkes.ArrivalWindow
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
	arrivals, ready, err := tradeExcitationSample.MeasureArrival(input)

	if err != nil || !ready {
		return ExcitationInput{}, ready, err
	}

	return arrivals.legacy(), true, nil
}

/*
MeasureArrival precomputes a typed stream while preserving legacy timestamp semantics.
*/
func (tradeExcitationSample *TradeExcitationSample) MeasureArrival(
	input TradeExcitationInput,
) (ExcitationArrivalInput, bool, error) {
	if input.Symbol == "" || (input.Side != "buy" && input.Side != "sell") {
		return ExcitationArrivalInput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"trade-excitation-sample: trade frame requires symbol and buy/sell side",
			nil,
		))
	}

	arrivalSecond := input.ArrivalSecond()

	if arrivalSecond <= 0 {
		return ExcitationArrivalInput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"trade-excitation-sample: timestamp required",
			nil,
		))
	}

	window := tradeExcitationSample.window(input.Symbol)
	arrival := secondToTime(arrivalSecond)

	if input.Side == "buy" {
		window.arrivals.AppendBuy(arrival)
		window.buySeconds = appendExcitationSecond(window.buySeconds, arrivalSecond)
	}

	if input.Side == "sell" {
		window.arrivals.AppendSell(arrival)
		window.sellSeconds = appendExcitationSecond(window.sellSeconds, arrivalSecond)
	}

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

	window := &tradeExcitationWindow{
		arrivals:    hawkes.NewArrivalWindow(excitationSampleSideCap),
		buySeconds:  make([]float64, 0, excitationSampleSideCap),
		sellSeconds: make([]float64, 0, excitationSampleSideCap),
	}
	tradeExcitationSample.windows.Store(symbol, window)

	return window
}

func (window *tradeExcitationWindow) features(
	symbol string,
) (ExcitationArrivalInput, bool, error) {
	stream := window.arrivals.Stream()
	eventCount := len(stream.BuyTimes()) + len(stream.SellTimes())

	// Emit a measurement from whatever the window holds — a signal must produce
	// a reading on every tick, never sit silent behind a warmup gate. With a
	// single event the "mean" is that event's own value; sample sufficiency is
	// communicated downstream as low confidence, not as an absent measurement.
	// The only hard floor is one event, which bounds() requires to index [0];
	// this is always satisfied because features() runs after the current trade
	// has been appended.
	if eventCount == 0 {
		return ExcitationArrivalInput{}, false, nil
	}

	first, last := window.bounds()
	span := time.Duration((last - first) * float64(time.Second))

	if span <= 0 {
		span = time.Second
	}

	return ExcitationArrivalInput{
		Symbol:         symbol,
		Horizon:        time.Unix(0, int64(last*float64(time.Second))),
		FitCooldown:    DeriveFitCooldown(span),
		TouchImbalance: window.touchImbalance,
		Stream:         stream,
		buySeconds:     window.buySeconds,
		sellSeconds:    window.sellSeconds,
		horizonSeconds: last,
	}, true, nil
}

func (input ExcitationArrivalInput) legacy() ExcitationInput {
	return ExcitationInput{
		Symbol:             input.Symbol,
		HorizonSeconds:     input.horizonSeconds,
		FitCooldownSeconds: input.FitCooldown.Seconds(),
		TouchImbalance:     input.TouchImbalance,
		BuySeconds:         append([]float64(nil), input.buySeconds...),
		SellSeconds:        append([]float64(nil), input.sellSeconds...),
	}
}

func (window *tradeExcitationWindow) bounds() (float64, float64) {
	first := 0.0
	last := 0.0

	if len(window.buySeconds) > 0 {
		first = window.buySeconds[0]
		last = window.buySeconds[0]
	}

	if len(window.buySeconds) == 0 {
		first = window.sellSeconds[0]
		last = window.sellSeconds[0]
	}

	for _, seconds := range [][]float64{window.buySeconds, window.sellSeconds} {
		for _, second := range seconds {
			if second < first {
				first = second
			}

			if second > last {
				last = second
			}
		}
	}

	return first, last
}

func appendExcitationSecond(seconds []float64, arrival float64) []float64 {
	if len(seconds) < excitationSampleSideCap {
		return append(seconds, arrival)
	}

	copy(seconds, seconds[1:])
	seconds[len(seconds)-1] = arrival

	return seconds
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

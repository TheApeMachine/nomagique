package quality

import (
	"sync"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/algorithm/book/flow"
	"github.com/theapemachine/nomagique/equation"
)

/*
Sample turns book, level3, and trade updates into book-quality inputs.
State is retained per symbol so concurrent symbols do not share book or gate history.
*/
type Sample struct {
	config  SampleConfig
	windows *sync.Map
}

/*
SampleConfig describes direct book-quality sampling gates.
*/
type SampleConfig struct {
	VacuumGate          flow.GateQuantileConfig
	ChurnGate           flow.GateQuantileConfig
	CancelQtyGate       flow.GateQuantileConfig
	LevelSizeGate       flow.GateQuantileConfig
	VacuumLowPercentile float64
}

/*
OrderEvent is one level3 order event.
*/
type OrderEvent struct {
	Event    string
	OrderID  string
	Price    float64
	Quantity float64
}

/*
Level3Input is one level3 update for a symbol.
*/
type Level3Input struct {
	Symbol string
	Bids   []OrderEvent
	Asks   []OrderEvent
}

/*
DefaultSampleConfig returns the default direct sampler config.
*/
func DefaultSampleConfig() SampleConfig {
	return SampleConfig{
		VacuumGate: flow.GateQuantileConfig{
			Percentile: 0.9,
			MinSamples: 3,
		},
		ChurnGate: flow.GateQuantileConfig{
			Percentile: 0.75,
			MinSamples: 3,
		},
		CancelQtyGate: flow.GateQuantileConfig{
			Percentile: 0.5,
			MinSamples: 3,
		},
		LevelSizeGate: flow.GateQuantileConfig{
			Percentile: 0.75,
			MinSamples: 3,
		},
		VacuumLowPercentile: 0.25,
	}
}

/*
NewSample returns a direct book-quality sampler.
*/
func NewSample(configs ...SampleConfig) *Sample {
	config := DefaultSampleConfig()

	if len(configs) > 0 {
		config = configs[0]
	}

	return &Sample{
		config:  config,
		windows: &sync.Map{},
	}
}

/*
MeasureBook observes one L2 book update and reports a confidence maturity
alongside the resulting book-quality input.
*/
func (sample *Sample) MeasureBook(
	input flow.BookInput,
) (equation.BookQualityInput, bool, float64, error) {
	if input.Symbol == "" {
		return equation.BookQualityInput{}, false, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"book-quality-sample: symbol required",
			nil,
		))
	}

	window := sample.window(input.Symbol)
	frame := Frame{}
	window.observeLevels(input.Bids, flow.SideBid, &frame)
	window.observeLevels(input.Asks, flow.SideAsk, &frame)
	output := window.finish(frame, false)

	return output, true, window.maturity(), nil
}

/*
MeasureLevel3 observes one L3 book update and reports a confidence maturity
alongside the resulting book-quality input.
*/
func (sample *Sample) MeasureLevel3(
	input Level3Input,
) (equation.BookQualityInput, bool, float64, error) {
	if input.Symbol == "" {
		return equation.BookQualityInput{}, false, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"book-quality-sample: symbol required",
			nil,
		))
	}

	window := sample.window(input.Symbol)
	frame := Frame{}
	window.observeOrderEvents(input.Bids, flow.SideBid, &frame)
	window.observeOrderEvents(input.Asks, flow.SideAsk, &frame)
	output := window.finish(frame, true)
	window.tradePrices = nil

	return output, true, window.maturity(), nil
}

/*
MeasureTrade observes one trade update, staging it for later L3 fill/cancel
corroboration, and reports the currently held book-quality snapshot. A trade
print carries no book delta of its own, so it is ready as soon as some book
state already exists (LastPrice > 0) — never suppressed to a dead end.
*/
func (sample *Sample) MeasureTrade(
	input flow.TradeInput,
) (equation.BookQualityInput, bool, float64, error) {
	if input.Symbol == "" {
		return equation.BookQualityInput{}, false, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"book-quality-sample: symbol required",
			nil,
		))
	}

	if input.Price <= 0 || input.Quantity <= 0 {
		return equation.BookQualityInput{}, false, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"book-quality-sample: trade price and quantity required",
			nil,
		))
	}

	window := sample.window(input.Symbol)
	window.tradePrices = append(window.tradePrices, input.Price)
	output := window.snapshot()

	return output, output.LastPrice > 0, window.maturity(), nil
}

func (sample *Sample) window(symbol string) *Window {
	existing, ok := sample.windows.Load(symbol)

	if ok {
		return existing.(*Window)
	}

	window := newWindow(sample.config)
	sample.windows.Store(symbol, window)

	return window
}

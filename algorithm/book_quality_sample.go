package algorithm

import (
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/equation"
)

/*
BookQualitySample turns book, level3, and trade updates into book-quality inputs.
State is retained per symbol so concurrent symbols do not share book or gate history.
*/
type BookQualitySample struct {
	config  BookQualitySampleConfig
	windows map[string]*bookQualityWindow
}

/*
BookQualitySampleConfig describes direct book-quality sampling gates.
*/
type BookQualitySampleConfig struct {
	VacuumGate          GateQuantileConfig
	ChurnGate           GateQuantileConfig
	CancelQtyGate       GateQuantileConfig
	LevelSizeGate       GateQuantileConfig
	VacuumLowPercentile float64
}

/*
BookQualityOrderEvent is one level3 order event.
*/
type BookQualityOrderEvent struct {
	Event    string
	OrderID  string
	Price    float64
	Quantity float64
}

/*
BookQualityLevel3Input is one level3 update for a symbol.
*/
type BookQualityLevel3Input struct {
	Symbol string
	Bids   []BookQualityOrderEvent
	Asks   []BookQualityOrderEvent
}

/*
DefaultBookQualitySampleConfig returns the default direct sampler config.
*/
func DefaultBookQualitySampleConfig() BookQualitySampleConfig {
	return BookQualitySampleConfig{
		VacuumGate: GateQuantileConfig{
			Percentile: 0.9,
			MinSamples: 3,
		},
		ChurnGate: GateQuantileConfig{
			Percentile: 0.75,
			MinSamples: 3,
		},
		CancelQtyGate: GateQuantileConfig{
			Percentile: 0.5,
			MinSamples: 3,
		},
		LevelSizeGate: GateQuantileConfig{
			Percentile: 0.75,
			MinSamples: 3,
		},
		VacuumLowPercentile: 0.25,
	}
}

/*
NewBookQualitySample returns a direct book-quality sampler.
*/
func NewBookQualitySample(configs ...BookQualitySampleConfig) *BookQualitySample {
	config := DefaultBookQualitySampleConfig()

	if len(configs) > 0 {
		config = configs[0]
	}

	return &BookQualitySample{
		config:  config,
		windows: map[string]*bookQualityWindow{},
	}
}

/*
MeasureBook observes one L2 book update.
*/
func (bookQualitySample *BookQualitySample) MeasureBook(
	input BookflowBookInput,
) (equation.BookQualityInput, bool, error) {
	if input.Symbol == "" {
		return equation.BookQualityInput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"book-quality-sample: symbol required",
			nil,
		))
	}

	window := bookQualitySample.window(input.Symbol)
	frame := bookQualityFrame{}
	window.observeLevels(input.Bids, SideBid, &frame)
	window.observeLevels(input.Asks, SideAsk, &frame)

	return window.finish(frame, false), true, nil
}

/*
MeasureLevel3 observes one L3 book update.
*/
func (bookQualitySample *BookQualitySample) MeasureLevel3(
	input BookQualityLevel3Input,
) (equation.BookQualityInput, bool, error) {
	if input.Symbol == "" {
		return equation.BookQualityInput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"book-quality-sample: symbol required",
			nil,
		))
	}

	window := bookQualitySample.window(input.Symbol)
	frame := bookQualityFrame{}
	window.observeOrderEvents(input.Bids, SideBid, &frame)
	window.observeOrderEvents(input.Asks, SideAsk, &frame)
	inputs := window.finish(frame, true)
	window.tradePrices = nil

	return inputs, true, nil
}

/*
MeasureTrade observes one trade update for later L3 fill/cancel classification.
*/
func (bookQualitySample *BookQualitySample) MeasureTrade(
	input BookflowTradeInput,
) (equation.BookQualityInput, bool, error) {
	if input.Symbol == "" {
		return equation.BookQualityInput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"book-quality-sample: symbol required",
			nil,
		))
	}

	if input.Price <= 0 || input.Quantity <= 0 {
		return equation.BookQualityInput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"book-quality-sample: trade price and quantity required",
			nil,
		))
	}

	window := bookQualitySample.window(input.Symbol)
	window.tradePrices = append(window.tradePrices, input.Price)

	return equation.BookQualityInput{}, false, nil
}

func (bookQualitySample *BookQualitySample) window(symbol string) *bookQualityWindow {
	existing, ok := bookQualitySample.windows[symbol]

	if ok {
		return existing
	}

	window := newBookQualityWindow(bookQualitySample.config)
	bookQualitySample.windows[symbol] = window

	return window
}

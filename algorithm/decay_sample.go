package algorithm

import (
	"sync"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/algorithm/book/flow"
	"github.com/theapemachine/nomagique/equation"
)

/*
DecaySample accumulates book and trade frames into the feature batch Decay
expects, routing each symbol to its own decayWindow.
*/
type DecaySample struct {
	windows map[string]*decayWindow
	mu      sync.RWMutex
}

/*
NewDecaySample returns a book/trade sampler for microstructure decay classification.
*/
func NewDecaySample() *DecaySample {
	return &DecaySample{
		windows: map[string]*decayWindow{},
	}
}

/*
MeasureBook observes one book update and returns decay input plus a
maturity fraction (0..1) reflecting how many observations have accumulated
so far. It is ready as soon as a valid book snapshot exists, from the very
first frame.
*/
func (decaySample *DecaySample) MeasureBook(
	input flow.BookInput,
) (equation.DecayInput, bool, float64, error) {
	if input.Symbol == "" {
		return equation.DecayInput{}, false, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"decay-sample: symbol required",
			nil,
		))
	}

	window := decaySample.window(input.Symbol)
	window.mu.Lock()
	defer window.mu.Unlock()

	if err := window.ingestBook(input); err != nil {
		return equation.DecayInput{}, false, 0, err
	}

	// After applying levels, a crossed/empty book must not re-emit the prior
	// feature snapshot as if this frame established new microstructure.
	if window.book.Mid() <= 0 || window.book.Spread() <= 0 {
		return equation.DecayInput{}, false, 0, nil
	}

	return window.features()
}

/*
MeasureTrade observes one trade update and returns decay input plus a
maturity fraction (0..1) reflecting how many observations have accumulated
so far.
*/
func (decaySample *DecaySample) MeasureTrade(
	input flow.TradeInput,
) (equation.DecayInput, bool, float64, error) {
	if input.Symbol == "" {
		return equation.DecayInput{}, false, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"decay-sample: symbol required",
			nil,
		))
	}

	if input.Price <= 0 || input.Quantity <= 0 ||
		(input.Side != "buy" && input.Side != "sell") {
		return equation.DecayInput{}, false, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"decay-sample: trade price, quantity, and side required",
			nil,
		))
	}

	window := decaySample.window(input.Symbol)
	window.mu.Lock()
	defer window.mu.Unlock()

	if err := window.ingestTrade(input); err != nil {
		return equation.DecayInput{}, false, 0, err
	}

	return window.features()
}

/*
window returns the existing symbol window or atomically installs its first one.
*/
func (decaySample *DecaySample) window(symbol string) *decayWindow {
	decaySample.mu.RLock()
	existing, ok := decaySample.windows[symbol]
	decaySample.mu.RUnlock()

	if ok {
		return existing
	}

	decaySample.mu.Lock()
	defer decaySample.mu.Unlock()

	existing = decaySample.windows[symbol]

	if existing != nil {
		return existing
	}

	window := newDecayWindow()
	decaySample.windows[symbol] = window

	return window
}

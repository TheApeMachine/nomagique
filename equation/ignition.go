package equation

import (
	"math"
	"time"

	"github.com/theapemachine/errnie"
)

/*
Ignition scores ticker lift, price precursor, spread compression, and exhaustion
on a volume clock. Each symbol derives its equal-volume target from the median
positive executed-volume advance it has actually observed, so neither wall time
nor a universal sampling cadence controls its bars.
*/
type Ignition struct {
	windows  map[string]*ignitionWindow
	capacity int
}

/*
IgnitionInput is one ticker observation. Volume is cumulative executed quantity
for the symbol, while At supplies the causal elapsed time used to measure the
rate of each empirically sized volume bar.
*/
type IgnitionInput struct {
	Symbol string
	Volume float64
	Last   float64
	Bid    float64
	Ask    float64
	At     time.Time
}

/*
IgnitionOutput contains direct ticker ignition scores. Spread is the live quote
spread refreshed every observation; the remaining scores are held from the most
recent closed volume bar so quote churn cannot fabricate price observations.
*/
type IgnitionOutput struct {
	Value       float64
	RVOL        float64
	Precursor   float64
	Spread      float64
	Compression float64
	Ignition    float64
	Trend       float64
	Exhaustion  float64
	Strength    float64
	Category    float64
}

/*
NewIgnition returns a volume-clock calculator whose per-symbol empirical
history is bounded by the market feed's explicit retention capacity.
*/
func NewIgnition(capacity int) *Ignition {
	return &Ignition{
		windows:  make(map[string]*ignitionWindow),
		capacity: capacity,
	}
}

/*
Measure advances one symbol's causal volume clock and reports its calibration
maturity. A volume bar closes only after the accumulated executed quantity
reaches the symbol's median positive volume advance and positive elapsed event
time exists; quote-only observations update the live spread without closing a
bar.
*/
func (ignition *Ignition) Measure(
	input IgnitionInput,
) (IgnitionOutput, bool, float64, error) {
	if ignition == nil || ignition.capacity <= 0 {
		return IgnitionOutput{}, false, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"ignition: positive baseline capacity required",
			nil,
		))
	}

	if err := input.validate(); err != nil {
		return IgnitionOutput{}, false, 0, err
	}

	return ignition.window(input.Symbol).observe(input)
}

/*
validate rejects malformed market observations before they can contaminate a
symbol's retained empirical scales.
*/
func (input IgnitionInput) validate() error {
	if input.Symbol == "" {
		return errnie.Error(errnie.Err(
			errnie.Validation,
			"ignition: symbol required",
			nil,
		))
	}

	for _, value := range []float64{
		input.Volume,
		input.Last,
		input.Bid,
		input.Ask,
	} {
		if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return errnie.Error(errnie.Err(
				errnie.Validation,
				"ignition: volume, last, bid, and ask must be finite and positive",
				nil,
			))
		}
	}

	if input.Ask <= input.Bid {
		return errnie.Error(errnie.Err(
			errnie.Validation,
			"ignition: ask must be above bid",
			nil,
		))
	}

	return nil
}

/*
window returns the retained state for one symbol, creating only that concrete
per-symbol responsibility when the symbol first appears.
*/
func (ignition *Ignition) window(symbol string) *ignitionWindow {
	existing, ok := ignition.windows[symbol]

	if ok {
		return existing
	}

	window := &ignitionWindow{capacity: ignition.capacity}
	ignition.windows[symbol] = window

	return window
}

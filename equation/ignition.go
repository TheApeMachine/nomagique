package equation

import (
	"math"
	"sync"
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
	mu       sync.RWMutex
	windows  map[string]*ignitionWindow
	capacity int
}

/*
IgnitionInput is one executed-trade observation. Volume is the positive executed
quantity of that event, while At supplies the causal elapsed time used to measure
the rate of each empirically sized volume bar.
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
IgnitionSideOutput contains one reciprocal price direction's volume-clock
families. Buy scores positive log returns; Sell scores the same observations
under reciprocal price, which is exactly the negative original log return.
*/
type IgnitionSideOutput struct {
	Value       float64 `json:"value"`
	RVOL        float64 `json:"rvol"`
	Precursor   float64 `json:"precursor"`
	Compression float64 `json:"compression"`
	Ignition    float64 `json:"ignition"`
	Trend       float64 `json:"trend"`
	Exhaustion  float64 `json:"exhaustion"`
	Strength    float64 `json:"strength"`
	Category    float64 `json:"category"`
}

/*
IgnitionOutput contains both reciprocal directional families. The top-level
fields retain the legacy wire contract and carry the stronger directional
family, while Spread remains the current executable quote spread.
*/
type IgnitionOutput struct {
	Value       float64            `json:"value"`
	RVOL        float64            `json:"rvol"`
	Precursor   float64            `json:"precursor"`
	Spread      float64            `json:"spread"`
	Compression float64            `json:"compression"`
	Ignition    float64            `json:"ignition"`
	Trend       float64            `json:"trend"`
	Exhaustion  float64            `json:"exhaustion"`
	Strength    float64            `json:"strength"`
	Category    float64            `json:"category"`
	Buy         IgnitionSideOutput `json:"buy"`
	Sell        IgnitionSideOutput `json:"sell"`
}

/*
Map returns scored labels for the grid.
*/
func (output *IgnitionOutput) Map() map[string]float64 {
	if output == nil {
		return nil
	}

	return map[string]float64{
		"rvol":        output.RVOL,
		"precursor":   output.Precursor,
		"spread":      output.Spread,
		"compression": output.Compression,
		"ignition":    output.Ignition,
		"trend":       output.Trend,
		"exhaustion":  output.Exhaustion,
		"value":       output.Value,
		"strength":    output.Strength,
	}
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
	ignition.mu.RLock()
	existing := ignition.windows[symbol]
	ignition.mu.RUnlock()

	if existing != nil {
		return existing
	}

	ignition.mu.Lock()
	defer ignition.mu.Unlock()

	if existing = ignition.windows[symbol]; existing != nil {
		return existing
	}

	window := &ignitionWindow{capacity: ignition.capacity}
	ignition.windows[symbol] = window

	return window
}

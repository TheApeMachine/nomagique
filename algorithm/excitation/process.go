package excitation

import (
	"math"
	"sort"
	"sync"
	"time"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/hawkes"
)

/*
Readiness separates directly observed arrival evidence from identified model
state. A successful fit is not forecast evidence until residual and
out-of-sample validation exists, while ModelUpdated identifies the events that
actually established a new parameter epoch.
*/
type Readiness struct {
	Observation  bool   `json:"observation"`
	Intensity    bool   `json:"intensity"`
	HawkesFit    bool   `json:"hawkes_fit"`
	ModelUpdated bool   `json:"model_updated"`
	Forecast     bool   `json:"forecast"`
	Reason       string `json:"reason"`
}

/*
Outcome contains the numerical state of a bivariate exponential Hawkes model.
It carries no market category or trading gate because those interpretations
belong to the consuming logic and strategy layers.
*/
type Outcome struct {
	Fit                             hawkes.BivariateFit `json:"fit"`
	ObservedFrom                    time.Time           `json:"observed_from"`
	At                              time.Time           `json:"at"`
	Horizon                         time.Duration       `json:"horizon"`
	FitObservedFrom                 time.Time           `json:"fit_observed_from"`
	FitAt                           time.Time           `json:"fit_at"`
	HawkesPoissonLogLikelihoodDelta float64             `json:"hawkes_poisson_log_likelihood_delta"`
	CrossSelfLogLikelihoodDelta     float64             `json:"cross_self_log_likelihood_delta"`
	ImmediateBuyOffspring           float64             `json:"immediate_buy_offspring"`
	ImmediateSellOffspring          float64             `json:"immediate_sell_offspring"`
	TotalBuyDescendants             float64             `json:"total_buy_descendants"`
	TotalSellDescendants            float64             `json:"total_sell_descendants"`	
	EventCount                      int                 `json:"event_count"`
	BuyEventCount                   int                 `json:"buy_event_count"`
	SellEventCount                  int                 `json:"sell_event_count"`
	BuyArrivalRate                  float64             `json:"buy_arrival_rate"`
	SellArrivalRate                 float64             `json:"sell_arrival_rate"`
	MinimumFitEvents                int                 `json:"minimum_fit_events"`
	Maturity                        float64             `json:"maturity"`
	Readiness                       Readiness           `json:"readiness"`
}

/*
Map returns scored labels for the grid.
*/
func (outcome *Outcome) Map() map[string]float64 {
	if outcome == nil {
		return nil
	}

	buy := outcome.TotalBuyDescendants
	sell := outcome.TotalSellDescendants
	asymmetry := 0.0

	if total := buy + sell; total > 0 {
		asymmetry = math.Abs(buy-sell) / total
	}

	return map[string]float64{
		"spectral_radius":          outcome.Fit.SpectralRadius,
		"immediate_buy_offspring":  outcome.ImmediateBuyOffspring,
		"immediate_sell_offspring": outcome.ImmediateSellOffspring,
		"total_buy_descendants":    buy,
		"total_sell_descendants":   sell,
		"buy_arrival_rate":         outcome.BuyArrivalRate,
		"sell_arrival_rate":        outcome.SellArrivalRate,
		"branching_asymmetry":      asymmetry,
		"maturity":                 outcome.Maturity,
	}
}

/*
Process owns symbol-local fit state for one serial event processor. Keeping
the mutable estimators with their single owner preserves event ordering without
adding a concurrent map that cannot protect estimator internals.
*/
type Process struct {
	mu      sync.RWMutex
	symbols map[string]*symbol
}

/*
NewProcess returns a numerical Hawkes excitation process with isolated state
for every symbol it observes.
*/
func NewProcess() *Process {
	return &Process{symbols: make(map[string]*symbol)}
}

/*
Measure estimates the current empirical arrival rate or Hawkes state from one
chronological marked stream.
*/
func (process *Process) Measure(input Input) (Outcome, bool, error) {
	if input.Symbol == "" || len(input.Stream.BuyTimes())+len(input.Stream.SellTimes()) == 0 {
		return Outcome{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"excitation: invalid arrival stream",
			nil,
		))
	}

	_, latest, _ := input.Stream.Bounds()

	if input.Horizon.IsZero() || input.Horizon.Before(latest) {
		return Outcome{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"excitation: horizon must include the latest arrival",
			nil,
		))
	}

	if input.ObservedFrom.IsZero() {
		input.ObservedFrom = input.Stream.ObservationOrigin()
	}

	input.Stream = input.Stream.WithObservationOrigin(input.ObservedFrom)

	sym := process.symbol(input.Symbol)
	sym.mu.Lock()
	defer sym.mu.Unlock()

	outcome, ready := sym.measure(
		input.Stream,
		input.Horizon,
	)

	return outcome, ready, nil
}

func (process *Process) symbol(symbolName string) *symbol {
	process.mu.RLock()
	model := process.symbols[symbolName]
	process.mu.RUnlock()

	if model != nil {
		return model
	}

	process.mu.Lock()
	defer process.mu.Unlock()

	if model = process.symbols[symbolName]; model != nil {
		return model
	}

	model = newSymbol()
	process.symbols[symbolName] = model

	return model
}

/*
Symbols returns every symbol that has produced at least one excitation outcome.
*/
func (process *Process) Symbols() []string {
	if process == nil {
		return nil
	}

	process.mu.RLock()
	defer process.mu.RUnlock()

	if len(process.symbols) == 0 {
		return nil
	}

	symbols := make([]string, 0, len(process.symbols))

	for symbolName := range process.symbols {
		symbols = append(symbols, symbolName)
	}

	sort.Strings(symbols)

	return symbols
}

/*
Outcome returns the latest measured Hawkes outcome for one symbol.
*/
func (process *Process) Outcome(symbolName string) (Outcome, bool) {
	if process == nil || symbolName == "" {
		return Outcome{}, false
	}

	process.mu.RLock()
	model := process.symbols[symbolName]
	process.mu.RUnlock()

	if model == nil || !model.lastReady {
		return Outcome{}, false
	}

	return model.lastOutcome, true
}

/*
AwaitFit waits for an in-flight asynchronous parameter fit for one symbol.
It returns whether a completed epoch is pending publication on the next Measure.
*/
func (process *Process) AwaitFit(symbolName string) bool {
	if process == nil || symbolName == "" {
		return false
	}

	process.mu.RLock()
	model := process.symbols[symbolName]
	process.mu.RUnlock()

	if model == nil {
		return false
	}

	return model.awaitFit()
}

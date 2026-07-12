package excitation

import (
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
	Observation  bool
	Intensity    bool
	HawkesFit    bool
	ModelUpdated bool
	Forecast     bool
	Reason       string
}

/*
Outcome contains the numerical state of a bivariate exponential Hawkes model.
It carries no market category or trading gate because those interpretations
belong to the consuming logic and strategy layers.
*/
type Outcome struct {
	Fit                             hawkes.BivariateFit
	ObservedFrom                    time.Time
	At                              time.Time
	Horizon                         time.Duration
	FitObservedFrom                 time.Time
	FitAt                           time.Time
	HawkesPoissonLogLikelihoodDelta float64
	CrossSelfLogLikelihoodDelta     float64
	ImmediateBuyOffspring           float64
	ImmediateSellOffspring          float64
	TotalBuyDescendants             float64
	TotalSellDescendants            float64
	EventCount                      int
	BuyEventCount                   int
	SellEventCount                  int
	BuyArrivalRate                  float64
	SellArrivalRate                 float64
	MinimumFitEvents                int
	Maturity                        float64
	Readiness                       Readiness
}

/*
Process owns symbol-local fit state for one serial event processor. Keeping
the mutable estimators with their single owner preserves event ordering without
adding a concurrent map that cannot protect estimator internals.
*/
type Process struct {
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
	if input.Symbol == "" ||
		len(input.Stream.BuyTimes())+len(input.Stream.SellTimes()) == 0 {
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

	outcome, ready := process.symbol(input.Symbol).measure(
		input.Stream,
		input.Horizon,
	)

	return outcome, ready, nil
}

func (process *Process) symbol(symbolName string) *symbol {
	model, ok := process.symbols[symbolName]

	if ok {
		return model
	}

	model = newSymbol()
	process.symbols[symbolName] = model

	return model
}

package store

import (
	"context"
	"math"
	"sync"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/runtime"
)

/*
RegionCount is Action (9) + Oracle×{Veto,Approve} (8×2) — see regimes.go Catalog.
*/
const RegionCount = 25

/*
enterFirst through oracleFirst bound action and oracle slices Observe returns
so desk tiers address fixed region keys without scanning Catalog.
*/
const (
	enterFirst  uint8 = 0
	enterLast   uint8 = 2
	exitFirst   uint8 = 3
	exitLast    uint8 = 5
	holdFirst   uint8 = 6
	holdLast    uint8 = 8
	oracleFirst uint8 = 9
)

/*
deskBoolBoost is the minimum brightness desk learned-oracle bools contribute.
SNM on the same pair can still win when movement kernels disagree.
*/
const deskBoolBoost = 0.55

/*
oracleMap maps desk bools onto veto/approve keys: volume, day, spread, book, peak.
*/
var oracleMap = []map[bool]uint8{
	{false: 9, true: 10},
	{false: 11, true: 12},
	{false: 13, true: 14},
	{false: 15, true: 16},
	{false: 23, true: 24},
}

/*
snmEpsilon floors the SNM denominator so a jump from 0 to a tiny value cannot
claim |SNM| ≈ 1.0. 1e-4 still let 0→0.005 read as ~0.98; 0.05 keeps micro
jiggles from alternating enter regions up to min_path_length.
*/
const snmEpsilon = 0.05

/*
predictionAlpha is the EMA weight for the energy baseline so Surprise keeps
tracking regime shifts instead of freezing as 1/N → 0.
*/
const predictionAlpha = 0.02

/*
pinCounts is Catalog metric width per region key — O(1) brightness divisor.
*/
var pinCounts = func() [RegionCount]int {
	var counts [RegionCount]int

	for _, regime := range Catalog {
		if int(regime.Key) >= RegionCount {
			continue
		}

		counts[regime.Key] = len(regime.Metrics)
	}

	return counts
}()

/*
Prior holds one signal source's last metrics and EMA energy for SNM on the next tick.
*/
type Prior struct {
	Metrics    map[string]float64
	Samples    int64
	Prediction float64
	Energy     float64
}

/*
oraclePairs is the number of veto/approve couples from R09 onward.
*/
const oraclePairs = int((RegionCount - oracleFirst) / 2)

/*
gridSymbol holds one symbol's source priors so Observe for BTC never waits
on ETH's scoring pass. lastOracle / lastOracleBright remember the last lit
APPROVE so an idle metric tick re-emits that reading instead of going dark.
Vetoes are not held: pinMove only fires on rising stress, so a faded veto
would otherwise sticky-block until an approve snm rises again.
*/
type gridSymbol struct {
	mu               sync.Mutex
	priors           map[string]*Prior
	lastOracle       [oraclePairs]uint8
	lastOracleBright [oraclePairs]float64
}

/*
Grid groups metrics by pin into multi-signal confluence regions.
*/
type Grid struct {
	*runtime.System
	symbols sync.Map
}

/*
NewGrid roots a runtime.System so Observe can refuse work after pipeline ERROR.
*/
func NewGrid(ctx context.Context) *Grid {
	return &Grid{
		System: runtime.NewSystem(ctx, "grid"),
	}
}

/*
symbol load-or-creates per-symbol priors so envelope.Metrics fan-in stays parallel.
*/
func (grid *Grid) symbol(name string) *gridSymbol {
	if loaded, ok := grid.symbols.Load(name); ok {
		return loaded.(*gridSymbol)
	}

	created := &gridSymbol{priors: make(map[string]*Prior)}
	actual, _ := grid.symbols.LoadOrStore(name, created)

	return actual.(*gridSymbol)
}

/*
Region is the grid's view of runtime.Region.
*/
type Region = runtime.Region

/*
Observe scores Catalog pins from metrics, applies desk oracle bools, then
returns enter (flat) or the stronger of hold/exit (open), plus one side of
each oracle pair. Metric brightness is tanh-squashed into (0,1); desk lamps
are hard 0/1. Tied metric oracles fail closed to veto.

Observe takes ownership of each sources[source] label map for this symbol —
callers must not reuse those inner maps after the call.
*/
func (grid *Grid) Observe(
	symbol string,
	sources map[string]map[string]float64,
	open bool,
	volume, day, spread, book, peak bool,
	peakKnown bool,
) ([]*Region, error) {
	if symbol == "" {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"[grid.Observe] empty symbol",
			nil,
		))
	}

	if err := grid.Error(); err != nil {
		return nil, errnie.Error(errnie.Err(
			errnie.Internal,
			"[grid.Observe] grid is in error state",
			err,
		))
	}

	regions := make([]*Region, RegionCount)

	for key := range uint8(RegionCount) {
		regions[key] = &Region{Symbol: symbol, Key: key}
	}

	lane := grid.symbol(symbol)
	lane.mu.Lock()

	for source, metrics := range sources {
		if len(metrics) == 0 {
			continue
		}

		prior := lane.priors[source]

		if prior == nil {
			lane.priors[source] = &Prior{Metrics: metrics}
			continue
		}

		if prior.Samples > 0 {
			prior.Prediction += predictionAlpha * (prior.Energy - prior.Prediction)
		}

		sourcePins, ok := regionMetricPins[source]

		if !ok {
			prior.Metrics = metrics
			prior.Samples++
			continue
		}

		energy := 0.0
		pinned := 0

		for label := range metrics {
			if _, ok := sourcePins[label]; !ok {
				continue
			}

			snm := snmFrom(prior, label, metrics)
			energy += snm * snm
			pinned++
		}

		if pinned > 0 {
			energy /= float64(pinned)
		}

		excitation := 1 + grid.Surprise(prior.Prediction, energy)

		for label := range metrics {
			targets := sourcePins[label]

			if len(targets) == 0 {
				continue
			}

			snm := snmFrom(prior, label, metrics)
			level := metrics[label]
			move := pinMove(snm, level) * excitation

			if move <= 0 {
				continue
			}

			for _, key := range targets {
				if key >= RegionCount {
					continue
				}

				pins := pinCounts[key]

				if pins == 0 {
					continue
				}

				region := regions[key]
				region.Deformation += move
				region.Counts++
				region.Brightness = math.Tanh(region.Deformation / float64(pins))
			}
		}

		prior.Metrics = metrics
		prior.Energy = energy
		prior.Samples++
	}

	lane.mu.Unlock()

	for idx, ok := range []bool{volume, day, spread, book, peak} {
		// Missing ticker peak must not force a hard veto — that sealed the
		// whole universe during Ticker/Balance REST outages.
		if idx == 4 && !peakKnown {
			continue
		}

		litKey := oracleMap[idx][ok]
		darkKey := oracleMap[idx][!ok]

		lit := regions[litKey]
		if lit.Brightness < deskBoolBoost {
			lit.Brightness = deskBoolBoost
		}

		regions[darkKey].Brightness = 0
		regions[darkKey].Deformation = 0
		regions[darkKey].Counts = 0
	}

	from, to := enterFirst, enterLast

	if open {
		// detector + (support − opposition) — opposition is not additive.
		hold := regions[holdFirst].Brightness +
			(regions[holdFirst+1].Brightness - regions[holdLast].Brightness)
		exit := regions[exitFirst].Brightness +
			(regions[exitFirst+1].Brightness - regions[exitLast].Brightness)

		if exit > hold {
			from, to = exitFirst, exitLast
		} else {
			from, to = holdFirst, holdLast
		}
	}

	out := make([]*Region, 0, 11)

	for key := from; key <= to; key++ {
		out = append(out, regions[key])
	}

	lane.mu.Lock()

	for veto := oracleFirst; veto+1 < RegionCount; veto += 2 {
		approve := veto + 1
		pair := int((veto - oracleFirst) / 2)

		switch {
		case regions[approve].Brightness > regions[veto].Brightness:
			lane.lastOracle[pair] = approve
			lane.lastOracleBright[pair] = regions[approve].Brightness
			out = append(out, regions[approve])
		case regions[veto].Brightness > 0:
			lane.lastOracle[pair] = 0
			lane.lastOracleBright[pair] = 0
			out = append(out, regions[veto])
		case lane.lastOracle[pair] == approve && lane.lastOracleBright[pair] > 0:
			held := regions[approve]
			held.Brightness = lane.lastOracleBright[pair]
			out = append(out, held)
		default:
			lane.lastOracle[pair] = 0
			lane.lastOracleBright[pair] = 0
			out = append(out, regions[veto])
		}
	}

	lane.mu.Unlock()

	return out, nil
}

/*
SNM (Self-Normalized Movement) takes the reference from the prior magnitude
of the same label and the movement itself.
*/
func (grid *Grid) SNM(
	source, symbol, label string, metrics map[string]float64,
) float64 {
	lane := grid.symbol(symbol)
	lane.mu.Lock()
	defer lane.mu.Unlock()

	return snmFrom(lane.priors[source], label, metrics)
}

/*
pinMove is the unsigned contribution of one metric on a catalog pin.
Bullish pins (detector, support, approve) take an upward move at a positive
level. Stress pins (opposition, veto) take rising stress. Catalog already
splits those metrics onto their own keys, so a drop toward zero must not
light either side the way |delta| did.
*/
func pinMove(snm, level float64) float64 {
	if snm <= 0 || level <= 0 {
		return 0
	}

	return snm
}

/*
snmFrom is the per-label movement ratio Observe uses to light Catalog pins.
*/
func snmFrom(prior *Prior, label string, metrics map[string]float64) float64 {
	if prior == nil || label == "" {
		return 0
	}

	cur, ok := metrics[label]

	if !ok {
		return 0
	}

	rawPrior, ok := prior.Metrics[label]

	if !ok {
		return 0
	}

	delta := cur - rawPrior
	denominator := math.Abs(rawPrior) + math.Abs(delta) + snmEpsilon

	return delta / denominator
}

/*
Surprise returns how much current metric energy deviates from the expected
regime baseline.
*/
func (grid *Grid) Surprise(prediction, energy float64) float64 {
	if prediction <= 0 {
		return energy
	}

	return math.Abs(energy - prediction)
}

/*
Maturity is sensor readiness from sample history and metric completeness.
*/
func (grid *Grid) Maturity(source, symbol string) float64 {
	lane := grid.symbol(symbol)
	lane.mu.Lock()
	defer lane.mu.Unlock()

	prior := lane.priors[source]

	if prior == nil || prior.Samples <= 0 {
		return 0
	}

	samplesCount := float64(prior.Samples)
	maturity := 1.0 - (1.0 / samplesCount)

	if expected := float64(len(regionMetricPins[source])); expected > 0 {
		actual := float64(len(prior.Metrics))

		if actual < expected {
			maturity *= (actual / expected)
		}
	}

	return maturity
}

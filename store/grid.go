package store

import (
	"context"
	"maps"
	"math"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/runtime"
)

/*
RegionCount is Action (9) + Oracle×{Veto,Approve} (8×2) — see regimes.go Catalog.
*/
const RegionCount = 25

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
claim |SNM| = 1.0.
*/
const snmEpsilon = 1e-4

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

type Prior struct {
	Metrics    map[string]float64
	Samples    int64
	Prediction float64
	Energy     float64
}

/*
Grid groups metrics by pin into multi-signal confluence regions.
*/
type Grid struct {
	*runtime.System
	Priors map[string]map[string]*Prior
}

func NewGrid(ctx context.Context) *Grid {
	return &Grid{
		System: runtime.NewSystem(ctx, "grid"),
		Priors: make(map[string]map[string]*Prior),
	}
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
*/
func (grid *Grid) Observe(
	symbol string,
	sources map[string]map[string]float64,
	open bool,
	volume, day, spread, book, peak bool,
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

	for source, metrics := range sources {
		if len(metrics) == 0 {
			continue
		}

		if grid.Priors[source] == nil {
			grid.Priors[source] = make(map[string]*Prior)
		}

		prior := grid.Priors[source][symbol]

		if prior == nil {
			grid.Priors[source][symbol] = &Prior{Metrics: maps.Clone(metrics)}
			continue
		}

		if prior.Samples > 0 {
			prior.Prediction += predictionAlpha * (prior.Energy - prior.Prediction)
		}

		sourcePins, ok := regionMetricPins[source]

		if !ok {
			prior.Metrics = maps.Clone(metrics)
			prior.Samples++
			continue
		}

		energy := 0.0
		pinned := 0

		for label := range metrics {
			if _, ok := sourcePins[label]; !ok {
				continue
			}

			snm := grid.SNM(source, symbol, label, metrics)
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

			move := math.Abs(grid.SNM(source, symbol, label, metrics)) * excitation

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

		prior.Metrics = maps.Clone(metrics)
		prior.Energy = energy
		prior.Samples++
	}

	for idx, ok := range []bool{volume, day, spread, book, peak} {
		key := oracleMap[idx][ok]
		other := oracleMap[idx][!ok]
		regions[key].Brightness = 1
		regions[other].Brightness = 0
		regions[other].Deformation = 0
		regions[other].Counts = 0
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

	for veto := oracleFirst; veto+1 < RegionCount; veto += 2 {
		approve := veto + 1

		if regions[approve].Brightness > regions[veto].Brightness {
			out = append(out, regions[approve])
			continue
		}

		out = append(out, regions[veto])
	}

	return out, nil
}

/*
SNM (Self-Normalized Movement) takes the reference from the prior magnitude
of the same label and the movement itself.
*/
func (grid *Grid) SNM(
	source, symbol, label string, metrics map[string]float64,
) float64 {
	prior := grid.Priors[source][symbol]

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
	prior := grid.Priors[source][symbol]

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

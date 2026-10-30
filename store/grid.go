package store

import (
	"context"
	"math"
	"slices"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/runtime"
)

type Prior struct {
	Signal     Signal
	Samples    int64
	Prediction float64
	Energy     float64
	Scales     []float64 // running MAD scale per metric label for SNM
}

// priorEMANeff is the effective window for prior energy prediction and MAD scales.
const priorEMANeff = 64.0

// enterOppositionLevelLabels are R02 metrics whose [0,1] level is chronic posture
// (starved tape), not only a surprise vs prior — enter phase only.
var enterOppositionLevelLabels = map[string]struct{}{
	"starved_tape": {},
}

/*
Grid maps pinned metrics for one symbol into 15 confluence regions and
maintains per-source priors for SNM and energy surprise.
*/
type Grid struct {
	*runtime.System
	Priors map[string]map[string]*Prior
}

/*
RegionCount is the number of confluence regions the pins map metrics into.
*/
const RegionCount = 15

func NewGrid(ctx context.Context) *Grid {
	return &Grid{
		System: runtime.NewSystem(ctx, "grid"),
		Priors: make(map[string]map[string]*Prior),
	}
}

/*
Region is one confluence region's evidence in a frame. Deformation is the
sum of |SNM| scaled by excitation (one plus that source's energy surprise).
Counts is how many pinned metrics landed here. Brightness is the mean of the
top two per-metric deformations (top-2 corroborated pooling), so crowded
regions are not diluted by arithmetic averaging. Excitation can push it above
1. The JSON tags are the dashboard contract: the Impulse Map renders the
grid's own accepted regions from the training frame's grid/regions layer.
*/
type Region struct {
	Symbol       string    `json:"symbol"`
	Key          uint8     `json:"key"`
	Deformation  float64   `json:"deformation"`
	Brightness   float64   `json:"brightness"`
	Counts       int       `json:"counts"`
	deformations []float64 `json:"-"`
}

type Signal struct {
	Source string
	Symbol string
	Labels []string
	Values []float64
}

/*
Observe applies this tick's metrics for one pair, updates priors, and returns
the regions those metrics landed in. When enter is true (symbol not holding a
lot), R02 starved_tape also honors bounded level so chronic thin tape lights
enter opposition without requiring a surprise spike.
*/
func (grid *Grid) Observe(
	symbol string,
	sources map[string]map[string]float64,
	enter bool,
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

	if len(sources) == 0 {
		return nil, nil
	}

	var slots [RegionCount]*Region
	regions := make([]*Region, 0, RegionCount)

	for source, labels := range sources {
		if len(labels) == 0 {
			continue
		}

		if err := grid.ingestSignal(&slots, &regions, source, symbol, labels, enter); err != nil {
			return nil, err
		}
	}

	for _, region := range regions {
		if region == nil || len(region.deformations) == 0 {
			continue
		}

		slices.SortFunc(region.deformations, func(a, b float64) int {
			switch {
			case b > a:
				return 1
			case b < a:
				return -1
			default:
				return 0
			}
		})

		k := min(2, len(region.deformations))
		sum := 0.0
		for i := 0; i < k; i++ {
			sum += region.deformations[i]
		}

		region.Brightness = sum / float64(k)
	}

	return regions, nil
}

func (grid *Grid) ensurePrior(source, symbol string) *Prior {
	if grid.Priors[source] == nil {
		grid.Priors[source] = make(map[string]*Prior)
	}

	prior := grid.Priors[source][symbol]
	if prior == nil {
		prior = &Prior{}
		grid.Priors[source][symbol] = prior
	}

	return prior
}

func (grid *Grid) ingestSignal(
	slots *[RegionCount]*Region,
	regions *[]*Region,
	source, symbol string,
	labels map[string]float64,
	enterPhase bool,
) error {
	names := make([]string, 0, len(labels))

	for label := range labels {
		names = append(names, label)
	}

	slices.Sort(names)

	values := make([]float64, len(names))
	for i, label := range names {
		values[i] = labels[label]
	}

	signal := Signal{
		Source: source,
		Symbol: symbol,
		Labels: names,
		Values: values,
	}

	pinned(symbol, signal)

	prior := grid.ensurePrior(source, symbol)

	if prior.Samples == 0 && len(prior.Signal.Labels) == 0 {
		prior.Signal = signal
	}

	if prior.Samples > 0 {
		alpha := 2.0 / (priorEMANeff + 1.0)
		prior.Prediction = (1-alpha)*prior.Prediction + alpha*prior.Energy
	}

	energy := 0.0

	for idx := range signal.Values {
		grid.updateScale(prior, idx, signal.Values[idx])
		snm := grid.SNM(prior, idx, signal.Values[idx])
		energy += snm * snm
	}

	excitation := 1 + grid.Surprise(prior.Prediction, energy)
	sourcePins, ok := regionMetricPins[source]

	if !ok {
		prior.Signal = signal
		prior.Energy = energy
		prior.Samples++
		return nil
	}

	for idx, label := range signal.Labels {
		targets := sourcePins[label]

		if len(targets) == 0 {
			continue
		}

		for _, key := range targets {
			if key >= RegionCount {
				continue
			}

			region := slots[key]

			if region == nil {
				region = &Region{Symbol: symbol, Key: key}
				slots[key] = region
				*regions = append(*regions, region)
			}

			val := signal.Values[idx]
			snmDef := math.Abs(grid.SNM(prior, idx, val)) * excitation
			def := enterOppositionLevelDeformation(
				key, label, val, excitation, snmDef, enterPhase,
			)
			region.Deformation += def
			region.deformations = append(region.deformations, def)
			region.Counts++
		}
	}

	prior.Signal = signal
	prior.Energy = energy
	prior.Samples++

	return nil
}

/*
pinned checks whether all labels in a signal are mapped in regionMetricPins.
Unpinned labels are logged as a warning and skipped during region accumulation,
rather than failing the entire observation for the symbol.
*/
func pinned(symbol string, signal Signal) {
	sourcePins, ok := regionMetricPins[signal.Source]

	if !ok {
		return
	}

	for _, label := range signal.Labels {
		if _, ok := sourcePins[label]; ok {
			continue
		}

		errnie.Warn("grid: " + symbol + " metric " + signal.Source + "/" + label + " is not pinned to a region; skipping")
	}
}

func (grid *Grid) updateScale(prior *Prior, idx int, val float64) {
	if prior == nil {
		return
	}

	for len(prior.Scales) <= idx {
		prior.Scales = append(prior.Scales, 0)
	}

	alpha := 2.0 / (priorEMANeff + 1.0)
	absVal := math.Abs(val)
	if prior.Scales[idx] <= 0 {
		prior.Scales[idx] = absVal
		return
	}

	prior.Scales[idx] = (1-alpha)*prior.Scales[idx] + alpha*absVal
}

/*
SNM (Self-Normalized Magnitude) soft-clips evidence with a running MAD scale
so high-volatility regimes stay rankable instead of saturating together.
*/
func (grid *Grid) SNM(prior *Prior, idx int, val float64) float64 {
	if val == 0 {
		return 0
	}

	scale := 1.0
	if prior != nil && idx < len(prior.Scales) && prior.Scales[idx] > 0 {
		scale = prior.Scales[idx]
	}

	return math.Tanh(val / scale)
}

/*
Surprise is the absolute gap between this tick's metric energy and the
running prediction. It is zero when they match. An empty prediction
returns the energy itself, which is a sum of squares and cannot be negative.
*/
func (grid *Grid) Surprise(prediction, energy float64) float64 {
	if prediction <= 0 {
		return energy
	}

	return math.Abs(energy - prediction)
}

func enterOppositionLevelDeformation(
	regionKey uint8,
	label string,
	val, excitation, snmDef float64,
	enterPhase bool,
) float64 {
	if !enterPhase || regionKey != 2 {
		return snmDef
	}
	if _, ok := enterOppositionLevelLabels[label]; !ok {
		return snmDef
	}
	if val < 0 || val > 1 {
		return snmDef
	}
	// Blend level gently so chronic starved tape lights R02 without swamping netSpark.
	levelDef := 0.5 * val * excitation
	if levelDef > snmDef {
		return levelDef
	}
	return snmDef
}

/*
SymbolMaturity is the mean grid maturity across enter-story sources that have
seen this symbol. A single cold source must not veto pairs that are warm on
pumpdump and depthflow.
*/
var enterMaturitySources = map[string]struct{}{
	"pumpdump":    {},
	"depthflow":   {},
	"cvd":         {},
	"correlation": {},
	"leadlag":     {},
}

func (grid *Grid) SymbolMaturity(symbol string) float64 {
	if grid == nil || symbol == "" || len(grid.Priors) == 0 {
		return 0
	}

	var sum float64
	var count int

	for source, bySymbol := range grid.Priors {
		if _, ok := enterMaturitySources[source]; !ok {
			continue
		}

		prior := bySymbol[symbol]
		if prior == nil || prior.Samples <= 0 {
			continue
		}

		signal := Signal{
			Source: source,
			Symbol: symbol,
			Labels: prior.Signal.Labels,
		}
		sum += grid.Maturity(&signal)
		count++
	}

	if count == 0 {
		return 0
	}

	return sum / float64(count)
}

/*
Maturity calculates sensor readiness based strictly on
empirical sample history and metric completeness (warm-up gate).
*/
func (grid *Grid) Maturity(signal *Signal) float64 {
	prior := grid.Priors[signal.Source][signal.Symbol]

	if prior == nil || prior.Samples <= 0 {
		return 0
	}

	samplesCount := float64(prior.Samples)

	maturity := 1.0 - (1.0 / samplesCount)

	if expected := float64(len(regionMetricPins[prior.Signal.Source])); expected > 0 {
		actual := float64(len(prior.Signal.Labels))

		if actual < expected {
			maturity *= (actual / expected)
		}
	}

	return maturity
}

package store

import (
	"context"
	"fmt"
	"math"
	"slices"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/runtime"
)

/*
RegionCount is Action (9) + Oracle×{Veto,Approve} (7×2) — see regimes.go Catalog.
*/
const RegionCount = 23

const (
	enterDetector = 0
	enterLast     = 2
	exitDetector  = 3
	exitLast      = 5
	holdDetector  = 6
	holdLast      = 8
	oracleFirst   = 9
)

/*
TargetPinsPerRegion is the designed confluence width used as the brightness
divisor so a solitary blip cannot outscore a full confluence.
*/
const TargetPinsPerRegion = 3.0

/*
snmEpsilon floors the SNM denominator so a jump from 0 to a tiny value cannot
claim |SNM| = 1.0.
*/
const snmEpsilon = 1e-4

type Prior struct {
	Signal     Signal
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

type Signal struct {
	Source string
	Symbol string
	Labels []string
	Values []float64
}

/*
Observe applies this tick's grouped metrics and returns the full region set
(keys 0..RegionCount-1) for the symbol.
*/
func (grid *Grid) Observe(
	symbol string, sources map[string]map[string]float64,
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
	for key := uint8(0); key < RegionCount; key++ {
		regions[key] = &Region{Symbol: symbol, Key: key}
	}

	for source, labels := range sources {
		if len(labels) == 0 {
			continue
		}

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

		if err := pinned(symbol, signal); err != nil {
			return nil, err
		}

		if grid.Priors[source] == nil {
			grid.Priors[source] = make(map[string]*Prior)
		}

		prior := grid.Priors[source][symbol]

		if prior == nil {
			grid.Priors[source][symbol] = &Prior{Signal: signal}
			continue
		}

		if prior.Samples > 0 {
			prior.Prediction += (prior.Energy - prior.Prediction) / float64(prior.Samples)
		}

		energy := 0.0

		for _, label := range signal.Labels {
			snm := grid.SNM(&signal, label)
			energy += snm * snm
		}

		if n := len(signal.Labels); n > 0 {
			energy /= float64(n)
		}

		excitation := 1 + grid.Surprise(prior.Prediction, energy)
		sourcePins, ok := regionMetricPins[source]

		if !ok {
			prior.Signal = signal
			prior.Energy = energy
			prior.Samples++
			continue
		}

		for _, label := range signal.Labels {
			targets := sourcePins[label]

			if len(targets) == 0 {
				continue
			}

			move := math.Abs(grid.SNM(&signal, label)) * excitation
			if move <= 0 {
				continue
			}

			for _, key := range targets {
				if key >= RegionCount {
					continue
				}

				region := regions[key]
				region.Deformation += move
				region.Counts++
				region.Brightness = region.Deformation / TargetPinsPerRegion
			}
		}

		prior.Signal = signal
		prior.Energy = energy
		prior.Samples++
	}

	return regions, nil
}

/*
Emit masks Observe output for the inventory regime and collapses each oracle
to the brighter of Veto/Approve. Priors stay intact; only published brightness
is gated. Support/Opposition stay lit — path resistance resolves them later.
*/
func (grid *Grid) Emit(regions []*Region, open bool) {
	if len(regions) == 0 {
		return
	}

	if open {
		zeroBand(regions, enterDetector, enterLast)
	} else {
		zeroBand(regions, exitDetector, exitLast)
		zeroBand(regions, holdDetector, holdLast)
	}

	for veto := uint8(oracleFirst); int(veto)+1 < len(regions) && veto < RegionCount; veto += 2 {
		approve := veto + 1
		left := regions[veto]
		right := regions[approve]

		if left == nil || right == nil {
			continue
		}

		if left.Brightness >= right.Brightness {
			right.Brightness = 0
			right.Deformation = 0
			right.Counts = 0
			continue
		}

		left.Brightness = 0
		left.Deformation = 0
		left.Counts = 0
	}
}

func zeroBand(regions []*Region, from, to uint8) {
	for key := from; key <= to && int(key) < len(regions); key++ {
		region := regions[key]
		if region == nil {
			continue
		}

		region.Brightness = 0
		region.Deformation = 0
		region.Counts = 0
	}
}

/*
Shine lights one region as a hard oracle/action hit (absolute gates).
*/
func Shine(region *Region) {
	if region == nil {
		return
	}

	region.Brightness = 1
	region.Deformation = TargetPinsPerRegion
	region.Counts = 1
}

/*
PathToken is the single dominant microstructural regime (R00–R08) after Emit.
Oracles stay on the board for Tradable(); they are not sequencer alphabet.
*/
func PathToken(regions []*Region) (token string, brightness float64) {
	if len(regions) == 0 {
		return "", 0
	}

	limit := min(len(regions), int(holdLast) + 1)

	var action *Region

	for key := uint8(0); key < uint8(limit); key++ {
		region := regions[key]
		if region == nil || region.Brightness <= 0 {
			continue
		}

		if action == nil || region.Brightness > action.Brightness {
			action = region
		}
	}

	if action == nil {
		return "", 0
	}

	return fmt.Sprintf("R%02d", action.Key), action.Brightness
}

/*
pinned rejects a label that the source map does not name. An unknown source
has no pins to check.
*/
func pinned(symbol string, signal Signal) error {
	sourcePins, ok := regionMetricPins[signal.Source]

	if !ok {
		return nil
	}

	for _, label := range signal.Labels {
		if _, ok := sourcePins[label]; ok {
			continue
		}

		return errnie.Err(
			errnie.Validation,
			"grid: "+symbol+" metric "+signal.Source+"/"+label+" is not pinned to a region",
			nil,
		)
	}

	return nil
}

/*
SNM (Self-Normalized Movement) takes the reference from the prior magnitude
of the same label and the movement itself.
*/
func (grid *Grid) SNM(signal *Signal, label string) float64 {
	prior := grid.Priors[signal.Source][signal.Symbol]

	if prior == nil || label == "" {
		return 0
	}

	curIdx := slices.Index(signal.Labels, label)
	if curIdx < 0 || curIdx >= len(signal.Values) {
		return 0
	}

	priorIdx := slices.Index(prior.Signal.Labels, label)
	if priorIdx < 0 || priorIdx >= len(prior.Signal.Values) {
		return 0
	}

	rawPrior := prior.Signal.Values[priorIdx]
	delta := signal.Values[curIdx] - rawPrior
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

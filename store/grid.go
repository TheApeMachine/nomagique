package store

import (
	"context"
	"math"
	"slices"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/runtime"
)

/*
RegionCount is the number of confluence regions the pins map metrics into.
*/
const RegionCount = 15

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

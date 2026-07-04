package equation

import (
	"math"
	"sort"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/probability"
	"gonum.org/v1/gonum/stat"
)

/*
Decay classifies book thinning, spread widening, pressure fade, and imbalance flip.
*/
type Decay struct{}

/*
DecayInput contains the float-only microstructure decay inputs.
*/
type DecayInput struct {
	LastPrice  float64
	BidDepths  []float64
	AskDepths  []float64
	Densities  []float64
	Spreads    []float64
	Pressures  []float64
	Imbalances []float64
}

/*
DecayOutput contains the float-only decay scores.
*/
type DecayOutput struct {
	Value      float64
	Strength   float64
	Mechanical float64
	Fragile    float64
	Thermal    float64
	Reversal   float64
	Urgency    float64
	Category   float64
}

/*
NewDecay returns a microstructure decay calculator.
*/
func NewDecay() *Decay {
	return &Decay{}
}

/*
Measure calculates decay scores from floats without artifact transport.
*/
func (decay *Decay) Measure(input DecayInput) (DecayOutput, error) {
	if input.LastPrice <= 0 {
		return DecayOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"decay: last price must be positive",
			nil,
		))
	}

	series := [][]float64{
		input.BidDepths,
		input.AskDepths,
		input.Densities,
		input.Spreads,
		input.Pressures,
		input.Imbalances,
	}

	longOutcome, err := decay.exitSide(series, 1)

	if err != nil {
		return DecayOutput{}, err
	}

	shortOutcome, err := decay.exitSide(series, -1)

	if err != nil {
		return DecayOutput{}, err
	}

	mechanical := longOutcome.mechanical
	fragile := longOutcome.fragile
	thermal := longOutcome.thermal
	reversal := longOutcome.reversal
	urgency := longOutcome.urgency
	category := longOutcome.category

	if shortOutcome.urgency > urgency {
		mechanical = shortOutcome.mechanical
		fragile = shortOutcome.fragile
		thermal = shortOutcome.thermal
		reversal = shortOutcome.reversal
		urgency = shortOutcome.urgency
		category = shortOutcome.category
	}

	return DecayOutput{
		Value:      urgency,
		Strength:   urgency,
		Mechanical: mechanical,
		Fragile:    fragile,
		Thermal:    thermal,
		Reversal:   reversal,
		Urgency:    urgency,
		Category:   float64(category),
	}, nil
}

type decaySideOutcome struct {
	mechanical float64
	fragile    float64
	thermal    float64
	reversal   float64
	urgency    float64
	category   int
}

func (decay *Decay) exitSide(series [][]float64, side int) (decaySideOutcome, error) {
	bidDepths := series[0]
	askDepths := series[1]
	densities := series[2]
	spreads := series[3]
	pressures := series[4]
	imbalances := series[5]

	thinning := 0.0
	fade := 0.0
	flip := 0.0

	if side > 0 {
		thinning = depthTrend(bidDepths)
		fade = pressureFade(pressures, 1)
		flip = imbalanceFlip(imbalances, 1)
	}

	if side < 0 {
		thinning = depthTrend(askDepths)
		fade = pressureFade(pressures, -1)
		flip = imbalanceFlip(imbalances, -1)
	}

	widen := spreadWiden(spreads)
	collapse := depthTrend(densities)
	thinningScore := 0.0
	collapseScore := 0.0
	fragileScore := 0.0
	thermalScore := 0.0
	reversalScore := 0.0
	var err error

	if thinning > 0 {
		thinningScore, err = probability.MagnitudeMargin(thinning)

		if err != nil {
			return decaySideOutcome{}, errnie.Error(errnie.Err(
				errnie.Validation,
				"equation decay: thinning margin failed",
				err,
			))
		}
	}

	if collapse > 0 {
		collapseScore, err = probability.MagnitudeMargin(collapse)

		if err != nil {
			return decaySideOutcome{}, errnie.Error(errnie.Err(
				errnie.Validation,
				"equation decay: collapse margin failed",
				err,
			))
		}
	}

	if widen > 0 {
		fragileScore, err = probability.MagnitudeMargin(widen)

		if err != nil {
			return decaySideOutcome{}, errnie.Error(errnie.Err(
				errnie.Validation,
				"equation decay: spread margin failed",
				err,
			))
		}
	}

	if fade > 0 {
		thermalScore, err = probability.MagnitudeMargin(fade)

		if err != nil {
			return decaySideOutcome{}, errnie.Error(errnie.Err(
				errnie.Validation,
				"equation decay: pressure margin failed",
				err,
			))
		}
	}

	if flip > 0 {
		reversalScore, err = probability.MagnitudeMargin(flip)

		if err != nil {
			return decaySideOutcome{}, errnie.Error(errnie.Err(
				errnie.Validation,
				"equation decay: imbalance margin failed",
				err,
			))
		}
	}

	mechanicalScore := math.Max(thinningScore, collapseScore)

	margins := []float64{
		mechanicalScore,
		fragileScore,
		thermalScore,
		reversalScore,
	}

	fusionWeights, err := probability.SoftmaxScoresNormalized(margins)

	if err != nil {
		return decaySideOutcome{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"equation decay: softmax fusion failed",
			err,
		))
	}

	urgency := 0.0

	for index, weight := range fusionWeights {
		urgency += weight * margins[index]
	}

	category := classifyDecay(mechanicalScore, fragileScore, thermalScore, reversalScore)

	return decaySideOutcome{
		mechanical: margins[0],
		fragile:    margins[1],
		thermal:    margins[2],
		reversal:   margins[3],
		urgency:    urgency,
		category:   category,
	}, nil
}

func depthTrend(depths []float64) float64 {
	if len(depths) < 4 {
		return 0
	}

	splitIndex := len(depths) / 2

	if splitIndex < 1 {
		return 0
	}

	recent := stat.Mean(depths[splitIndex:], nil)
	prior := stat.Mean(depths[:splitIndex], nil)

	if prior <= 0 {
		return 0
	}

	return (prior - recent) / prior
}

func spreadWiden(spreads []float64) float64 {
	if len(spreads) < 4 {
		return 0
	}

	sorted := append([]float64(nil), spreads...)
	sort.Float64s(sorted)

	median := stat.Quantile(0.5, stat.LinInterp, sorted, nil)
	current := spreads[len(spreads)-1]

	if median <= 0 || current <= median {
		return 0
	}

	return (current - median) / median
}

func pressureFade(pressures []float64, side int) float64 {
	if len(pressures) < 3 {
		return 0
	}

	recent := pressures[len(pressures)-1]
	prior := pressures[0]

	for _, value := range pressures[:len(pressures)-1] {
		if side > 0 && value > prior {
			prior = value
		}

		if side < 0 && value < prior {
			prior = value
		}
	}

	if side > 0 {
		if prior <= 0 || recent >= prior {
			return 0
		}

		return (prior - recent) / math.Abs(prior)
	}

	if prior >= 0 || recent <= prior {
		return 0
	}

	return (recent - prior) / math.Abs(prior)
}

func imbalanceFlip(imbalances []float64, side int) float64 {
	if len(imbalances) < 2 {
		return 0
	}

	recent := imbalances[len(imbalances)-1]
	prior := stat.Mean(imbalances[:len(imbalances)-1], nil)

	if side > 0 && prior > 0 && recent < 0 {
		return math.Abs(recent) / prior
	}

	if side < 0 && prior < 0 && recent > 0 {
		return recent / math.Abs(prior)
	}

	return 0
}

func classifyDecay(thinning, widen, fade, flip float64) int {
	best := thinning
	category := 1

	if widen > best {
		best = widen
		category = 2
	}

	if fade > best {
		best = fade
		category = 3
	}

	if flip > best {
		best = flip
		category = 4
	}

	if best <= 0 {
		return 0
	}

	return category
}

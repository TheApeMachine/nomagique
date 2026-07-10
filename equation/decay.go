package equation

import (
	"math"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/probability"
)

/*
Decay classifies book thinning, spread widening, pressure fade, and imbalance flip.
*/
type Decay struct{}

/*
DecayInput carries pre-computed, per-tick microstructure decay ingredients.
Every field is produced by an adaptive nomagique/statistic primitive that is
already well-defined from its first observation, so there are no raw
histories and no per-metric sample-count floors here.
*/
type DecayInput struct {
	LastPrice float64

	// BidDepthRatio, AskDepthRatio, and DensityRatio are short-window mean
	// over long-window median depth ratios (statistic.MeanMedianRatio). A
	// ratio below 1 means recent depth sits below its own baseline.
	BidDepthRatio float64
	AskDepthRatio float64
	DensityRatio  float64

	// SpreadDeviation is the current spread's rolling z-score against its
	// own history (statistic.RollingZScore). Positive means wider than typical.
	SpreadDeviation float64

	// Pressure is the current smoothed trade pressure. PressurePeak and
	// PressureTrough are the running max/min of pressure including this
	// tick (statistic.Max/Min), so a tick that sets a new extreme reports
	// zero fade by construction — there is nothing to fade from yet.
	Pressure       float64
	PressurePeak   float64
	PressureTrough float64

	// Imbalance is the current book imbalance. PriorImbalanceMean is the
	// mean of every imbalance observed strictly before this tick
	// (statistic.Mean); it is zero on the first tick, since nothing
	// precedes it to compare against.
	Imbalance          float64
	PriorImbalanceMean float64
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
Measure calculates decay scores from pre-computed microstructure scalars.
*/
func (decay *Decay) Measure(input DecayInput) (DecayOutput, error) {
	if input.LastPrice <= 0 {
		return DecayOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"decay: last price must be positive",
			nil,
		))
	}

	longOutcome, err := decay.exitSide(input, 1)

	if err != nil {
		return DecayOutput{}, err
	}

	shortOutcome, err := decay.exitSide(input, -1)

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

func (decay *Decay) exitSide(input DecayInput, side int) (decaySideOutcome, error) {
	thinning := 0.0
	fade := 0.0
	flip := 0.0

	if side > 0 {
		thinning = ratioDecline(input.BidDepthRatio)
		fade = pressureFade(input.Pressure, input.PressurePeak, 1)
		flip = imbalanceFlip(input.Imbalance, input.PriorImbalanceMean, 1)
	}

	if side < 0 {
		thinning = ratioDecline(input.AskDepthRatio)
		fade = pressureFade(input.Pressure, input.PressureTrough, -1)
		flip = imbalanceFlip(input.Imbalance, input.PriorImbalanceMean, -1)
	}

	widen := math.Max(0, input.SpreadDeviation)
	collapse := ratioDecline(input.DensityRatio)
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

/*
ratioDecline turns a short-over-long ratio into a decline fraction. A ratio
at or above 1 means recent activity is not below its own baseline, so there
is nothing to report.
*/
func ratioDecline(ratio float64) float64 {
	if ratio <= 0 || ratio >= 1 {
		return 0
	}

	return 1 - ratio
}

/*
pressureFade compares the current pressure reading to the running extremum
(peak for the long side, trough for the short side) computed including this
tick. A tick that sets a new extreme is, by construction, not faded from it.
*/
func pressureFade(pressure, extremum float64, side int) float64 {
	if side > 0 {
		if extremum <= 0 || pressure >= extremum {
			return 0
		}

		return (extremum - pressure) / math.Abs(extremum)
	}

	if extremum >= 0 || pressure <= extremum {
		return 0
	}

	return (pressure - extremum) / math.Abs(extremum)
}

/*
imbalanceFlip reports how far the current imbalance has crossed against a
prior mean that supported the opposite side. It is zero unless the prior
mean actually favored this side and the current reading has flipped sign.
*/
func imbalanceFlip(current, priorMean float64, side int) float64 {
	if side > 0 && priorMean > 0 && current < 0 {
		return math.Abs(current) / priorMean
	}

	if side < 0 && priorMean < 0 && current > 0 {
		return current / math.Abs(priorMean)
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

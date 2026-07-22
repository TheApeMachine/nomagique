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
Every ratio and deviation is scaled against the symbol's own accumulated
history, while price rejection joins pressure fade on the current observation.
*/
type DecayInput struct {
	LastPrice float64

	// PriceReturn is the latest book-mid log return. Thermal exhaustion
	// requires this price move to reject the held side; pressure fade alone
	// cannot establish that an advancing position is actually exhausting.
	PriceReturn float64

	// BidDepthRatio, AskDepthRatio, and DensityRatio are the current depth
	// divided by that series' accumulated mean. A ratio below 1 means the
	// current book sits below its own observed baseline.
	BidDepthRatio float64
	AskDepthRatio float64
	DensityRatio  float64

	// SpreadDeviation is the current spread's standardized deviation from its
	// accumulated mean. Positive means wider than the symbol's observed norm.
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
DecaySideOutput contains one held side's float-only decay scores. Long scores
describe evidence for exiting a bought position; short scores describe evidence
for exiting a sold position.
*/
type DecaySideOutput struct {
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
DecayOutput preserves both hypothetical position sides so a market-wide signal
does not discard one side before the consumer identifies its actual position.
*/
type DecayOutput struct {
	Long  DecaySideOutput
	Short DecaySideOutput
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
	values := []float64{
		input.LastPrice,
		input.PriceReturn,
		input.BidDepthRatio,
		input.AskDepthRatio,
		input.DensityRatio,
		input.SpreadDeviation,
		input.Pressure,
		input.PressurePeak,
		input.PressureTrough,
		input.Imbalance,
		input.PriorImbalanceMean,
	}

	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return DecayOutput{}, errnie.Error(errnie.Err(
				errnie.Validation,
				"decay: every input must be finite",
				nil,
			))
		}
	}

	if input.LastPrice <= 0 || input.BidDepthRatio <= 0 ||
		input.AskDepthRatio <= 0 || input.DensityRatio <= 0 {
		return DecayOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"decay: price and depth ratios must be positive",
			nil,
		))
	}

	longOutcome, err := decay.exitSide(input, decayLong)

	if err != nil {
		return DecayOutput{}, err
	}

	shortOutcome, err := decay.exitSide(input, decayShort)

	if err != nil {
		return DecayOutput{}, err
	}

	return DecayOutput{
		Long:  longOutcome,
		Short: shortOutcome,
	}, nil
}

/*
decaySide identifies which held position the equation is evaluating without
leaking an application-specific order-side type into nomagique.
*/
type decaySide int

const (
	decayShort decaySide = -1
	decayLong  decaySide = 1
)

/*
exitSide scores the depth, spread, pressure, and imbalance evidence that is
adverse to one held side.
*/
func (decay *Decay) exitSide(
	input DecayInput, side decaySide,
) (DecaySideOutput, error) {
	thinning := 0.0
	fade := 0.0
	flip := 0.0

	if side == decayLong {
		thinning = ratioDecline(input.BidDepthRatio)
		fade = pressureFade(input.Pressure, input.PressurePeak, decayLong) *
			priceRejection(input.PriceReturn, side)
		flip = imbalanceFlip(input.Imbalance, input.PriorImbalanceMean, decayLong)
	}

	if side == decayShort {
		thinning = ratioDecline(input.AskDepthRatio)
		fade = pressureFade(input.Pressure, input.PressureTrough, decayShort) *
			priceRejection(input.PriceReturn, side)
		flip = imbalanceFlip(input.Imbalance, input.PriorImbalanceMean, decayShort)
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
			return DecaySideOutput{}, errnie.Error(errnie.Err(
				errnie.Validation,
				"equation decay: thinning margin failed",
				err,
			))
		}
	}

	if collapse > 0 {
		collapseScore, err = probability.MagnitudeMargin(collapse)

		if err != nil {
			return DecaySideOutput{}, errnie.Error(errnie.Err(
				errnie.Validation,
				"equation decay: collapse margin failed",
				err,
			))
		}
	}

	if widen > 0 {
		fragileScore, err = probability.MagnitudeMargin(widen)

		if err != nil {
			return DecaySideOutput{}, errnie.Error(errnie.Err(
				errnie.Validation,
				"equation decay: spread margin failed",
				err,
			))
		}
	}

	if fade > 0 {
		thermalScore, err = probability.MagnitudeMargin(fade)

		if err != nil {
			return DecaySideOutput{}, errnie.Error(errnie.Err(
				errnie.Validation,
				"equation decay: pressure margin failed",
				err,
			))
		}
	}

	if flip > 0 {
		reversalScore, err = probability.MagnitudeMargin(flip)

		if err != nil {
			return DecaySideOutput{}, errnie.Error(errnie.Err(
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
		return DecaySideOutput{}, errnie.Error(errnie.Err(
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

	return DecaySideOutput{
		Value:      urgency,
		Strength:   urgency,
		Mechanical: margins[0],
		Fragile:    margins[1],
		Thermal:    margins[2],
		Reversal:   margins[3],
		Urgency:    urgency,
		Category:   float64(category),
	}, nil
}

/*
ratioDecline turns a current-over-baseline ratio into a decline fraction. A
ratio at or above 1 means current depth is not below its own baseline, so there
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
func pressureFade(pressure, extremum float64, side decaySide) float64 {
	if side == decayLong {
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
priceRejection returns the adverse log-return magnitude for a held position.
A long is rejected only by a negative move, while a short is rejected only by
a positive move.
*/
func priceRejection(priceReturn float64, side decaySide) float64 {
	if side == decayLong {
		return math.Max(0, -priceReturn)
	}

	return math.Max(0, priceReturn)
}

/*
imbalanceFlip reports how far the current imbalance has crossed against a
prior mean that supported the opposite side. It is zero unless the prior
mean actually favored this side and the current reading has flipped sign.
*/
func imbalanceFlip(current, priorMean float64, side decaySide) float64 {
	if side == decayLong && priorMean > 0 && current < 0 {
		return math.Abs(current) / priorMean
	}

	if side == decayShort && priorMean < 0 && current > 0 {
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

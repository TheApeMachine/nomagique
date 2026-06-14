package learning

import (
	"fmt"
	"math"
)

/*
FeedbackTuner applies top-down prediction feedback to a surprise threshold and
classifier weights exactly once per newer sample set for a matching symbol.

Tuning rates derive from the current sample count and forecast scale.
*/
type FeedbackTuner struct {
	lastSamples int
}

/*
NewFeedbackTuner creates a tuner with no prior application history.
*/
func NewFeedbackTuner() *FeedbackTuner {
	return &FeedbackTuner{}
}

/*
Apply adjusts threshold and logits when feedback arrives for a newer sample window.
*/
func (tuner *FeedbackTuner) Apply(
	symbol string,
	feedbackSymbol string,
	samples int,
	mse float64,
	scale float64,
	bias float64,
	weights *ClassifierWeights,
) (applied bool, err error) {
	if samples <= 0 {
		return false, nil
	}

	if feedbackSymbol != symbol {
		return false, nil
	}

	if samples <= tuner.lastSamples {
		return false, nil
	}

	if weights == nil {
		return false, fmt.Errorf("learning: FeedbackTuner requires weights")
	}

	if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		return false, fmt.Errorf("learning: FeedbackTuner scale must be finite and positive")
	}

	tuner.lastSamples = samples

	mseGain := 1.0 / float64(samples)
	adjustedThreshold := weights.Threshold

	if mse > 0 {
		adjustedThreshold += mse * mseGain
	}

	thresholdSpread := math.Max(mse, math.SmallestNonzeroFloat64)
	weights.Threshold = clamp(
		adjustedThreshold,
		weights.Threshold/(1.0+thresholdSpread),
		weights.Threshold*(1.0+thresholdSpread),
	)

	learningRate := scale / float64(samples)
	adjustment := bias * learningRate
	orgShare := 1.0 / 3.0
	exShare := 0.5

	weights.WIgnVol += adjustment
	weights.WIgnPrec += adjustment
	weights.WCoilComp += adjustment
	weights.WCoilPrec += adjustment
	weights.WOrgPrec += adjustment
	weights.WOrgComp += adjustment * orgShare
	weights.WOrgVol += adjustment * orgShare
	weights.WExVol -= adjustment * exShare
	weights.WExPrec -= adjustment * exShare
	weights.clamp()

	return true, nil
}

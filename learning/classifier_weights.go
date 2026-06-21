package learning

import (
	"fmt"
	"math"

	"github.com/theapemachine/errnie"
)

/*
ClassifierFeatureScales holds typical magnitudes for classifier inputs.

Pass observed medians or means from live history so logits are balanced by
inverse scale — larger-magnitude features receive smaller weights so each term
contributes comparably at typical operating points.
*/
type ClassifierFeatureScales struct {
	RVol        float64
	Precursor   float64
	Compression float64
}

/*
ClassifierWeights holds softmax logits coefficients for a four-way classifier.

Each class score is a weighted sum of normalized features. Weights are derived
from ClassifierFeatureScales at construction and clamped relative to those scales
during feedback tuning.
*/
type ClassifierWeights struct {
	Threshold float64
	scales    ClassifierFeatureScales
	WIgnVol   float64
	WIgnPrec  float64
	WCoilComp float64
	WCoilPrec float64
	WOrgPrec  float64
	WOrgComp  float64
	WOrgVol   float64
	WExVol    float64
	WExPrec   float64
}

/*
NewClassifierWeights builds balanced logits from a surprise threshold and
observed feature scales.

Each weight is the reciprocal of its feature scale, normalized within its class
so terms sum to one. Non-positive scales return an error.
*/
func NewClassifierWeights(
	threshold float64,
	scales ClassifierFeatureScales,
) (ClassifierWeights, error) {
	if threshold <= 0 {
		return ClassifierWeights{}, errnie.Error(fmt.Errorf(
			"learning: NewClassifierWeights threshold must be positive, got %v",
			threshold,
		))
	}

	rvolScale, err := positiveScale(scales.RVol, "rvol")

	if err != nil {
		return ClassifierWeights{}, err
	}

	precursorScale, err := positiveScale(scales.Precursor, "precursor")

	if err != nil {
		return ClassifierWeights{}, err
	}

	compressionScale, err := positiveScale(scales.Compression, "compression")

	if err != nil {
		return ClassifierWeights{}, err
	}

	ignVol := 1.0 / rvolScale
	ignPrec := 1.0 / precursorScale
	ignSum := ignVol + ignPrec

	coilComp := 1.0 / compressionScale
	coilPrec := 1.0 / precursorScale
	coilSum := coilComp + coilPrec

	orgPrec := 1.0 / precursorScale
	orgComp := 1.0 / compressionScale
	orgVol := 1.0 / rvolScale
	orgSum := orgPrec + orgComp + orgVol

	exVol := 1.0 / rvolScale
	exPrec := 1.0 / precursorScale
	exSum := exVol + exPrec

	return ClassifierWeights{
		Threshold: threshold,
		scales:    scales,
		WIgnVol:   ignVol / ignSum,
		WIgnPrec:  ignPrec / ignSum,
		WCoilComp: coilComp / coilSum,
		WCoilPrec: coilPrec / coilSum,
		WOrgPrec:  orgPrec / orgSum,
		WOrgComp:  orgComp / orgSum,
		WOrgVol:   orgVol / orgSum,
		WExVol:    exVol / exSum,
		WExPrec:   exPrec / exSum,
	}, nil
}

func (weights *ClassifierWeights) FeatureScales() ClassifierFeatureScales {
	return weights.scales
}

func positiveScale(scale float64, name string) (float64, error) {
	if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		return 0, errnie.Error(fmt.Errorf(
			"learning: NewClassifierWeights %s scale must be finite and positive, got %v",
			name,
			scale,
		))
	}

	return scale, nil
}

/*
Scores returns four class logits for the given normalized features.
Each input is divided by its observed scale before the class formulas run so
(1.0 - feature) terms stay meaningful relative to typical operating points.
*/
func (weights *ClassifierWeights) Scores(
	rvol, precursor, compression float64,
) []float64 {
	rvolNorm := normalizeFeature(rvol, weights.scales.RVol)
	precursorNorm := normalizeFeature(precursor, weights.scales.Precursor)
	compressionNorm := normalizeFeature(compression, weights.scales.Compression)

	return []float64{
		rvolNorm*weights.WIgnVol + precursorNorm*weights.WIgnPrec,
		compressionNorm*weights.WCoilComp + (1.0-precursorNorm)*weights.WCoilPrec,
		precursorNorm*weights.WOrgPrec + (1.0-compressionNorm)*weights.WOrgComp + rvolNorm*weights.WOrgVol,
		(1.0-rvolNorm)*weights.WExVol + (1.0-precursorNorm)*weights.WExPrec,
	}
}

/*
Strength returns the ignition-class logit without compression.
*/
func (weights *ClassifierWeights) Strength(rvol, precursor float64) float64 {
	rvolNorm := normalizeFeature(rvol, weights.scales.RVol)
	precursorNorm := normalizeFeature(precursor, weights.scales.Precursor)

	return rvolNorm*weights.WIgnVol + precursorNorm*weights.WIgnPrec
}

func normalizeFeature(value, scale float64) float64 {
	ratio := value

	if scale > 0 && !math.IsNaN(scale) && !math.IsInf(scale, 0) {
		ratio = value / scale
	}

	return squashFeature(ratio)
}

func squashFeature(value float64) float64 {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}

	return value / (1.0 + value)
}

func (weights *ClassifierWeights) clamp() {
	floor, ceiling := weightBounds(weights.scales)
	weights.WIgnVol = clamp(weights.WIgnVol, floor, ceiling)
	weights.WIgnPrec = clamp(weights.WIgnPrec, floor, ceiling)
	weights.WCoilComp = clamp(weights.WCoilComp, floor, ceiling)
	weights.WCoilPrec = clamp(weights.WCoilPrec, floor, ceiling)
	weights.WOrgPrec = clamp(weights.WOrgPrec, floor, ceiling)
	weights.WOrgComp = clamp(weights.WOrgComp, floor, ceiling)
	weights.WOrgVol = clamp(weights.WOrgVol, floor, ceiling)
	weights.WExVol = clamp(weights.WExVol, floor, ceiling)
	weights.WExPrec = clamp(weights.WExPrec, floor, ceiling)
}

func weightBounds(scales ClassifierFeatureScales) (float64, float64) {
	minScale := math.Min(scales.RVol, math.Min(scales.Precursor, scales.Compression))
	maxScale := math.Max(scales.RVol, math.Max(scales.Precursor, scales.Compression))

	floor := 1.0 / (maxScale * 10.0)
	ceiling := 10.0 / minScale

	return floor, ceiling
}

func clamp(value, lower, upper float64) float64 {
	return math.Min(math.Max(value, lower), upper)
}

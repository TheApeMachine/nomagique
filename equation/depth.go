package equation

import (
	"math"
	"sort"

	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
Depth ranks quote volume against peer quartiles with optional baseline scaling.
*/
type Depth struct{}

/*
DepthInput contains the float-only liquidity inputs.
*/
type DepthInput struct {
	QuoteVolume    float64
	Peers          []float64
	RelativeVolume float64
	BaselineReady  bool
}

/*
DepthOutput contains the float-only liquidity scores.
*/
type DepthOutput struct {
	Value         float64
	ScarcityScore float64
	MedianScore   float64
	DepthScore    float64
	Strength      float64
	Category      float64
}

/*
NewDepth returns a cross-section liquidity depth calculator.
*/
func NewDepth() *Depth {
	return &Depth{}
}

/*
Measure calculates depth scores from floats without artifact transport.
*/
func (depth *Depth) Measure(input DepthInput) (DepthOutput, error) {
	if len(input.Peers) < 2 {
		return DepthOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"depth: insufficient peer count",
			nil,
		))
	}

	if math.IsNaN(input.QuoteVolume) || math.IsInf(input.QuoteVolume, 0) {
		return DepthOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"depth: quote volume must be finite",
			nil,
		))
	}

	sortedPeers := append([]float64(nil), input.Peers...)
	sort.Float64s(sortedPeers)

	lower := stat.Quantile(0.25, stat.LinInterp, sortedPeers, nil)
	upper := stat.Quantile(0.75, stat.LinInterp, sortedPeers, nil)
	median := stat.Quantile(0.5, stat.LinInterp, sortedPeers, nil)

	if median <= 0 {
		return DepthOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"depth: peer median must be positive",
			nil,
		))
	}

	peakScarcity := isPeakScarcity(input.QuoteVolume, input.Peers)
	historicallyLiquid := depthHistoricallyLiquid(
		input.RelativeVolume,
		input.BaselineReady,
		input.Peers,
		input.QuoteVolume,
	)

	category := classifyDepth(
		input.QuoteVolume,
		lower,
		upper,
		peakScarcity,
		historicallyLiquid,
	)
	if category == 0 {
		return DepthOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"depth: unclassified liquidity state",
			nil,
		))
	}

	scarcityScore := math.Max(0, (median-input.QuoteVolume)/median)
	depthScore := math.Max(0, (input.QuoteVolume-median)/median)
	medianScore := medianDepthEvidence(input.QuoteVolume, lower, upper)
	strength := scarcityScore

	if category == 3 {
		strength = depthScore
	}

	if category == 2 {
		strength = math.Max(scarcityScore, medianScore)
	}

	if strength <= 0 && category != 2 {
		return DepthOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"depth: non-positive strength",
			nil,
		))
	}

	return DepthOutput{
		Value:         strength,
		ScarcityScore: scarcityScore,
		MedianScore:   medianScore,
		DepthScore:    depthScore,
		Strength:      strength,
		Category:      float64(category),
	}, nil
}

func classifyDepth(
	quoteVol, lower, upper float64,
	peakScarcity bool,
	historicallyLiquid bool,
) int {
	if historicallyLiquid && (peakScarcity || quoteVol <= lower) {
		return 2
	}

	if peakScarcity || quoteVol <= lower {
		return 1
	}

	if quoteVol >= upper {
		return 3
	}

	return 2
}

func isPeakScarcity(quoteVol float64, volumes []float64) bool {
	if len(volumes) == 0 {
		return false
	}

	minVolume := volumes[0]

	for _, volume := range volumes[1:] {
		if volume < minVolume {
			minVolume = volume
		}
	}

	return quoteVol <= minVolume
}

func medianDepthEvidence(quoteVol, lower, upper float64) float64 {
	if upper <= lower || quoteVol <= lower || quoteVol >= upper {
		return 0
	}

	center := (lower + upper) / 2
	halfBand := (upper - lower) / 2

	if halfBand <= 0 {
		return 0
	}

	distance := math.Abs(quoteVol - center)

	return math.Max(0, 1-distance/halfBand)
}

func depthHistoricallyLiquid(
	relativeVolume float64,
	ready bool,
	peers []float64,
	quoteVol float64,
) bool {
	if !ready || quoteVol <= 0 || len(peers) < 2 {
		return false
	}

	sortedPeers := append([]float64(nil), peers...)
	sort.Float64s(sortedPeers)
	median := stat.Quantile(0.5, stat.LinInterp, sortedPeers, nil)

	if median <= 0 {
		return false
	}

	peerRelatives := make([]float64, len(peers))

	for index, peerVolume := range peers {
		peerRelatives[index] = peerVolume / median
	}

	sortedRelatives := append([]float64(nil), peerRelatives...)
	sort.Float64s(sortedRelatives)
	liquidThreshold := stat.Quantile(0.75, stat.LinInterp, sortedRelatives, nil)

	return relativeVolume >= liquidThreshold
}

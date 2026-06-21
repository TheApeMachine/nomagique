package equation

import (
	"io"
	"math"
	"sort"

	"github.com/theapemachine/datura"
	"gonum.org/v1/gonum/stat"
)

/*
Depth ranks quote volume against peer quartiles with optional baseline scaling.

Payload layout: scaledQuoteVol, peerCount, peer volumes, relativeVolume,
baselineReady (0/1).
*/
type Depth struct {
	artifact *datura.Artifact
}

/*
NewDepth returns a cross-section liquidity depth stage.
*/
func NewDepth() io.ReadWriteCloser {
	return &Depth{
		artifact: datura.Acquire("depth", datura.APPJSON),
	}
}

func (depth *Depth) Write(p []byte) (int, error) {
	depth.artifact.WithPayload(p)
	return len(p), nil
}

func (depth *Depth) Read(p []byte) (int, error) {
	state, err := stageState(depth.artifact.DecryptPayload())

	if err != nil {
		return 0, err
	}

	batch := Features(state)

	if len(batch) < 2 {
		return emitZero(state, p)
	}

	scaledQuoteVol := batch[0]
	peerCount := int(batch[1])

	if peerCount < 2 || len(batch) < 2+peerCount+2 {
		return emitZero(state, p)
	}

	peers := append([]float64(nil), batch[2:2+peerCount]...)
	relativeVolume := batch[2+peerCount]
	baselineReady := batch[3+peerCount] > 0

	sortedPeers := append([]float64(nil), peers...)
	sort.Float64s(sortedPeers)

	lower := stat.Quantile(0.25, stat.LinInterp, sortedPeers, nil)
	upper := stat.Quantile(0.75, stat.LinInterp, sortedPeers, nil)
	median := stat.Quantile(0.5, stat.LinInterp, sortedPeers, nil)

	if median <= 0 {
		return emitZero(state, p)
	}

	peakScarcity := isPeakScarcity(scaledQuoteVol, peers)
	historicallyLiquid := depthHistoricallyLiquid(
		relativeVolume,
		baselineReady,
		peers,
		scaledQuoteVol,
	)

	category := classifyDepth(
		scaledQuoteVol,
		lower,
		upper,
		peakScarcity,
		historicallyLiquid,
	)

	if category == 0 {
		return emitZero(state, p)
	}

	scarcityRaw := math.Max(0, (median-scaledQuoteVol)/median)
	depthRaw := math.Max(0, (scaledQuoteVol-median)/median)

	scarcityScore := scarcityRaw

	if peakScarcity {
		scarcityScore = math.Max(scarcityScore, 1)
	}

	medianScore := medianDepthEvidence(scaledQuoteVol, lower, upper)
	strength := scarcityRaw

	if category == 3 {
		strength = depthRaw
	}

	if category == 2 {
		strength = math.Max(scarcityScore, medianScore)
	}

	if strength <= 0 && category != 2 {
		return emitZero(state, p)
	}

	return emitOutput(state, p, datura.Map[float64]{
		"value":         strength,
		"scarcityScore": scarcityScore,
		"medianScore":   medianScore,
		"depthScore":    depthRaw,
		"strength":      strength,
		"category":      float64(category),
	})
}

func (depth *Depth) Close() error {
	return nil
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

/*
AbsoluteScaledVolumes lifts peer volumes when baseline-relative volume is elevated.
*/
func AbsoluteScaledVolumes(
	quoteVol float64,
	peers []float64,
	relativeVolume float64,
	baselineReady bool,
) (float64, []float64) {
	absoluteScale := 1.0

	if baselineReady && relativeVolume > 0 {
		absoluteScale = math.Max(1.0, relativeVolume)
	}

	scaledPeers := make([]float64, len(peers))

	for index, peerVolume := range peers {
		scaledPeers[index] = peerVolume * absoluteScale
	}

	return quoteVol * absoluteScale, scaledPeers
}

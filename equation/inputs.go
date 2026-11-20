package equation

import "github.com/theapemachine/errnie"

/*
BookQualityInputKeys names the feature frame contract algorithm/book/quality
samples before Measure and Map fan into symm resonance keys.
*/
var BookQualityInputKeys = []string{
	"cancelBid",
	"fillBid",
	"cancelAsk",
	"fillAsk",
	"bidDepth",
	"askDepth",
	"toxicNear",
	"toxicBluffStrength",
	"fillToCancelThreshold",
	"churnGate",
	"supportGate",
	"vacuumStrengthCap",
	"lastPrice",
}

/*
FlowInputKeys names trade-pressure scalars trade_flow sampling must supply for
flow Map outputs on the grid.
*/
var FlowInputKeys = []string{
	"buyNotional",
	"sellNotional",
	"tradeCount",
	"grossFloor",
	"medianNotional",
}

/*
DepthInputKeys names quote-volume features depth sampling attaches before
scarcity and median scores export.
*/
var DepthInputKeys = []string{
	"scaledQuoteVol",
	"peerCount",
}

/*
ConvictionInputKeys names cross-section breadth fields conviction sampling must
align with surge and slump classification.
*/
var ConvictionInputKeys = []string{
	"breadth",
	"change",
	"surgeThreshold",
	"leader",
}

/*
ManifoldInputKeys names pressure and coherence scalars manifold sampling uses
before herd and drift scores reach the envelope.
*/
var ManifoldInputKeys = []string{
	"pressureGradNorm",
	"coherenceMag2",
	"guidanceSpeed",
	"viscosityProxy",
	"price",
}

/*
BookflowInputKeys names depth skew histories book-flow sampling retains so
spoof and loaded gates in Measure match live book state.
*/
var BookflowInputKeys = []string{
	"weighted",
	"level1",
	"flat",
	"flatOK",
	"mid",
	"spread",
	"touchDepth",
	"tradePressure",
	"weightedCount",
	"level1Count",
	"flatCount",
}

/*
DecayInputKeys names rolling depth and pressure counts decay sampling must
preserve for dual-sided exit urgency Map families.
*/
var DecayInputKeys = []string{
	"lastPrice",
	"bidDepthCount",
	"askDepthCount",
	"densityCount",
	"spreadCount",
	"pressureCount",
	"imbalanceCount",
}

/*
CohortInputKeys names cross-section correlation layout cohort sampling packs
into the feature frame before classifyCohort runs.
*/
var CohortInputKeys = []string{
	"window",
	"pairCorrelationCount",
	"peerCorrelationCount",
	"peerEnergyCount",
	"barSpacingSeconds",
	"energy",
}

/*
LagInputKeys names anchor lag features lag sampling exports for pairwise
lead-lag evidence in resonance.
*/
var LagInputKeys = []string{
	"isAnchor",
	"price",
	"moveReady",
	"moveMoved",
	"stallMargin",
	"lagOK",
	"lagBars",
	"lagCorr",
	"contempOK",
	"contempCorr",
	"sampleCount",
}

/*
FeatureFields reads named scalars in order; missing keys return validation errors.
*/
func FeatureFields(frame FeatureFrame, keys []string) ([]float64, error) {
	return frame.FeatureFields(keys)
}

/*
FeatureSlice reads a contiguous segment from the features vector.
*/
func FeatureSlice(frame FeatureFrame, offset, count int) ([]float64, error) {
	if count < 0 || offset < 0 || offset+count > len(frame.Features) {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"equation: feature slice out of range",
			nil,
		))
	}

	return frame.FeatureSlice(offset, count)
}

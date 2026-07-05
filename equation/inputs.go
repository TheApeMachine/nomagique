package equation

import "github.com/theapemachine/errnie"

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

var FluidflowInputKeys = []string{
	"reynolds",
	"divergence",
	"viscosity",
	"midAddRate",
	"midExecuteRate",
	"laminarCeiling",
	"turbulentFloor",
	"turbulentReady",
	"divergenceEdge",
	"icebergScore",
	"vorticity",
	"turbulence",
	"memory",
	"price",
	"spreadBPS",
	"changePct",
	"volume",
}

var FlowInputKeys = []string{
	"buyNotional",
	"sellNotional",
	"tradeCount",
	"grossFloor",
	"medianNotional",
}

var DepthInputKeys = []string{
	"scaledQuoteVol",
	"peerCount",
}

var ConvictionInputKeys = []string{
	"breadth",
	"change",
	"surgeThreshold",
	"leader",
}

var ManifoldInputKeys = []string{
	"pressureGradNorm",
	"coherenceMag2",
	"guidanceSpeed",
	"viscosityProxy",
	"price",
}

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

var DecayInputKeys = []string{
	"lastPrice",
	"bidDepthCount",
	"askDepthCount",
	"densityCount",
	"spreadCount",
	"pressureCount",
	"imbalanceCount",
}

var CohortInputKeys = []string{
	"window",
	"pairCorrelationCount",
	"peerCorrelationCount",
	"peerEnergyCount",
	"barSpacingSeconds",
	"energy",
}

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

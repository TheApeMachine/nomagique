package equation

import (
	"github.com/bytedance/sonic"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/statistic"
)

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
func FeatureFields(state *datura.Artifact, keys []string) ([]float64, error) {
	values := make([]float64, len(keys))

	for index, key := range keys {
		value, err := statistic.FeatureColumn(state, key)

		if err != nil {
			return nil, err
		}

		values[index] = value
	}

	return values, nil
}

/*
EnsureFeatureSchema stamps root/inputs from config when the wire payload omitted them.
*/
func EnsureFeatureSchema(state, config *datura.Artifact, defaultKeys []string) []string {
	inputKeys := datura.Peek[[]string](state, "inputs")

	if len(inputKeys) == 0 {
		inputKeys = datura.Peek[[]string](config, "inputs")

		if len(inputKeys) == 0 {
			inputKeys = defaultKeys
		}

		state.Poke(inputKeys, "inputs")
	}

	if datura.Peek[string](state, "root") == "" {
		state.Poke("features", "root")
	}

	return inputKeys
}

/*
FeatureSlice reads a contiguous segment from the features vector.
*/
func FeatureSlice(state *datura.Artifact, offset, count int) ([]float64, error) {
	features := Features(state)

	if count < 0 || offset+count > len(features) {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"equation: feature slice out of range",
			nil,
		))
	}

	return append([]float64(nil), features[offset:offset+count]...), nil
}

/*
MarshalFeatureSchema encodes semantic features for pipeline wire tests.
*/
func MarshalFeatureSchema(inputs []string, values []float64) []byte {
	payload := errnie.Does(func() ([]byte, error) {
		return sonic.Marshal(datura.Map[any]{
			"features": values,
			"inputs":   inputs,
			"root":     "features",
		})
	}).Or(func(err error) {
		errnie.Error(errnie.Err(errnie.IO, "equation: marshal feature schema payload", err))
	}).Value()

	return payload
}

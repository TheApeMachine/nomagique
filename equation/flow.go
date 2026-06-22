package equation

import (
	"io"
	"math"

	"github.com/theapemachine/datura"
)

/*
Flow classifies signed trade pressure against price response in a rolling window.
The constructor artifact holds schema inputs; Write buffers inbound wire on its payload.
*/
type Flow struct {
	artifact *datura.Artifact
}

/*
NewFlow returns a CVD flow stage wired from config attributes.
*/
func NewFlow(artifact *datura.Artifact) io.ReadWriteCloser {
	if artifact == nil {
		artifact = datura.Acquire("flow", datura.APPJSON)
	}

	if len(datura.Peek[[]string](artifact, "inputs")) == 0 {
		artifact.Poke(FlowInputKeys, "inputs")
	}

	return &Flow{
		artifact: artifact,
	}
}

func (flow *Flow) Write(p []byte) (int, error) {
	flow.artifact.WithPayload(p)
	return len(p), nil
}

func (flow *Flow) Read(p []byte) (int, error) {
	state, err := stageState(flow.artifact.DecryptPayload())

	if err != nil {
		return 0, err
	}

	inputKeys := ensureFeatureSchema(state, flow.artifact, FlowInputKeys)

	fields, err := featureFields(state, inputKeys)

	if err != nil || len(fields) < len(FlowInputKeys) {
		return rejectStage(state, "equation: invalid stage input")
	}

	buyNotional := fields[0]
	sellNotional := fields[1]
	tradeCount := int(fields[2])
	grossFloor := fields[3]
	medianNotional := fields[4]

	prices, err := featureSlice(state, len(inputKeys), len(Features(state))-len(inputKeys))

	if err != nil {
		return rejectStage(state, "equation: invalid stage input")
	}

	gross := buyNotional + sellNotional

	if gross <= 0 || tradeCount < 2 || len(prices) < 2 {
		return rejectStage(state, "equation: invalid stage input")
	}

	net := buyNotional - sellNotional
	netFraction := math.Abs(net) / gross
	firstPrice := prices[0]
	lastPrice := prices[len(prices)-1]

	if firstPrice <= 0 || lastPrice <= 0 {
		return rejectStage(state, "equation: invalid stage input")
	}

	if medianNotional <= 0 || math.IsNaN(medianNotional) || math.IsInf(medianNotional, 0) {
		return rejectStage(state, "equation: medianNotional must be positive")
	}

	priceResponseBps := math.Abs(lastPrice/firstPrice-1) * basisPointsPerUnit
	flowPressure := gross / medianNotional

	if flowPressure <= 0 || math.IsNaN(flowPressure) || math.IsInf(flowPressure, 0) {
		return rejectStage(state, "equation: invalid stage input")
	}

	impactEfficiency := priceResponseBps / flowPressure
	priceDrift := lastPrice - firstPrice
	flatThreshold := medianAbsoluteMove(prices)
	driveThreshold := 1 / math.Sqrt(float64(tradeCount))

	highNet := netFraction >= driveThreshold
	flowAligned := (net > 0 && priceDrift > 0) || (net < 0 && priceDrift < 0)
	hiddenAbsorption := flowHiddenAbsorption(
		highNet, flowPressure, impactEfficiency, priceDrift, flatThreshold, tradeCount,
	)
	flatPrice := hiddenAbsorption || flowFlatPrice(priceDrift, flatThreshold, tradeCount)

	category := flowCategory(
		highNet, flowAligned, flatPrice, gross, grossFloor, tradeCount, hiddenAbsorption,
	)

	absorption := 0.0
	drive := 0.0
	balance := math.Max(0, 1-netFraction)
	starvation := 0.0

	if hiddenAbsorption {
		absorption = netFraction * (1 / (1 + impactEfficiency))
	}

	if highNet && flowAligned && !flatPrice {
		drive = netFraction
	}

	if highNet && !flatPrice && !flowAligned {
		absorption = math.Max(absorption, netFraction)
	}

	if category == flowCategoryStarvation {
		if grossFloor > 0 && gross < grossFloor {
			starvation = math.Max(0, 1-gross/grossFloor)
		}

		if tradeCount < 3 && !highNet {
			starvation = 1 - float64(tradeCount)/3
		}
	}

	strength := math.Max(absorption, math.Max(drive, math.Max(balance, starvation)))

	return emitOutput(state, p, datura.Map[float64]{
		"value":       strength,
		"absorption":  absorption,
		"drive":       drive,
		"balance":     balance,
		"starvation":  starvation,
		"net":         net,
		"netFraction": netFraction,
		"category":    float64(category),
	})
}

func (flow *Flow) Close() error {
	return nil
}

const (
	flowCategoryStarvation = 4
	basisPointsPerUnit     = 10000
)

func medianAbsoluteMove(prices []float64) float64 {
	moves := priceMoves(prices)

	if len(moves) == 0 {
		return 0
	}

	absoluteMoves := make([]float64, len(moves))

	for index, move := range moves {
		absoluteMoves[index] = math.Abs(move)
	}

	return medianOf(absoluteMoves)
}

func medianOf(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sorted := append([]float64(nil), values...)
	sortFloat64s(sorted)

	mid := len(sorted) / 2

	if len(sorted)%2 == 1 {
		return sorted[mid]
	}

	return (sorted[mid-1] + sorted[mid]) / 2
}

func sortFloat64s(values []float64) {
	for left := 1; left < len(values); left++ {
		pivot := values[left]
		right := left - 1

		for right >= 0 && values[right] > pivot {
			values[right+1] = values[right]
			right--
		}

		values[right+1] = pivot
	}
}

func priceMoves(prices []float64) []float64 {
	moves := make([]float64, 0, len(prices)-1)

	for index := 1; index < len(prices); index++ {
		moves = append(moves, prices[index]-prices[index-1])
	}

	return moves
}

func flowFlatPrice(priceDrift, stepThreshold float64, tradeCount int) bool {
	if stepThreshold <= 0 {
		return priceDrift == 0
	}

	windowThreshold := stepThreshold * math.Sqrt(float64(tradeCount))

	return math.Abs(priceDrift) <= windowThreshold
}

func flowCategory(
	highNet, flowAligned, flatPrice bool,
	gross, grossFloor float64,
	tradeCount int,
	hiddenAbsorption bool,
) int {
	if grossFloor > 0 && gross < grossFloor {
		return flowCategoryStarvation
	}

	if tradeCount < 3 && !highNet {
		return flowCategoryStarvation
	}

	if hiddenAbsorption {
		return 1
	}

	if highNet && flowAligned {
		return 2
	}

	if highNet && !flatPrice && !flowAligned {
		return 1
	}

	return 3
}

func flowHiddenAbsorption(
	highNet bool,
	flowPressure float64,
	impactEfficiency float64,
	priceDrift float64,
	stepThreshold float64,
	tradeCount int,
) bool {
	if !highNet || flowPressure <= 0 {
		return false
	}

	if impactEfficiency <= 0 {
		return true
	}

	return flowPressure >= 1 && impactEfficiency <= 1 &&
		flowFlatPrice(priceDrift, stepThreshold, tradeCount)
}

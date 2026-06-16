package algorithm

import (
	"math"

	"github.com/theapemachine/datura"
)

const flowPayloadHeader = 5

/*
FlowOutcome holds cumulative volume delta scores derived from a trade window.
*/
type FlowOutcome struct {
	Absorption  float64
	Drive       float64
	Balance     float64
	Starvation  float64
	Net         float64
	NetFraction float64
}

/*
Flow classifies signed trade pressure against price response in a rolling window.

Payload layout: buyNotional, sellNotional, tradeCount, grossFloor, medianNotional,
then every observed price in window order.
*/
type Flow struct {
	artifact *datura.Artifact
	outcome  FlowOutcome
}

/*
NewFlow returns a CVD flow stage for io.ReadWriter pipelines.
*/
func NewFlow() *Flow {
	return &Flow{
		artifact: datura.Acquire("flow", datura.Artifact_Type_json),
	}
}

func (flow *Flow) Write(p []byte) (int, error) {
	return flow.artifact.Write(p)
}

func (flow *Flow) Read(p []byte) (int, error) {
	rehydrateArtifact(&flow.artifact, "flow", datura.Artifact_Type_json)

	payload, err := flow.artifact.Payload()

	if err == nil {
		flow.outcome = flow.evaluate(payloadSamples(payload))
		flow.publishReadings()
	}

	return flow.artifact.Read(p)
}

func (flow *Flow) Close() error {
	return nil
}

func (flow *Flow) evaluate(batch []float64) FlowOutcome {
	if len(batch) < flowPayloadHeader+2 {
		return FlowOutcome{}
	}

	buyNotional := batch[0]
	sellNotional := batch[1]
	tradeCount := int(batch[2])
	grossFloor := batch[3]
	medianNotional := batch[4]
	prices := batch[flowPayloadHeader:]

	gross := buyNotional + sellNotional

	if gross <= 0 || tradeCount < 2 || len(prices) < 2 {
		return FlowOutcome{}
	}

	net := buyNotional - sellNotional
	netFraction := math.Abs(net) / gross
	firstPrice := prices[0]
	lastPrice := prices[len(prices)-1]

	if firstPrice <= 0 || lastPrice <= 0 {
		return FlowOutcome{}
	}

	priceResponseBps := math.Abs(lastPrice/firstPrice-1) * 10000
	flowPressure := gross / math.Max(medianNotional, 1e-9)
	impactEfficiency := priceResponseBps / math.Max(flowPressure, 1e-9)
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

	outcome := FlowOutcome{
		Net:         net,
		NetFraction: netFraction,
		Balance:     math.Max(0, 1-netFraction),
	}

	if hiddenAbsorption {
		outcome.Absorption = netFraction * (1 / (1 + impactEfficiency))
	}

	if highNet && flowAligned && !flatPrice {
		outcome.Drive = netFraction
	}

	if highNet && !flatPrice && !flowAligned {
		outcome.Absorption = math.Max(outcome.Absorption, netFraction)
	}

	if category == flowCategoryStarvation {
		if grossFloor > 0 && gross < grossFloor {
			outcome.Starvation = math.Max(0, 1-gross/grossFloor)
		}

		if tradeCount < 3 && !highNet {
			outcome.Starvation = 1 - float64(tradeCount)/3
		}
	}

	return outcome
}

func (flow *Flow) publishReadings() {
	pokeFloat(flow.artifact, "flow.absorption", flow.outcome.Absorption)
	pokeFloat(flow.artifact, "flow.drive", flow.outcome.Drive)
	pokeFloat(flow.artifact, "flow.balance", flow.outcome.Balance)
	pokeFloat(flow.artifact, "flow.starvation", flow.outcome.Starvation)
	pokeFloat(flow.artifact, "flow.net_fraction", flow.outcome.NetFraction)
}

/*
AbsorptionReading returns hidden-absorption score as a composable classifier input.
*/
func (flow *Flow) AbsorptionReading() *FlowReading {
	return newFlowReading(flow, func(outcome FlowOutcome) float64 {
		return outcome.Absorption
	})
}

/*
DriveReading returns aggressive-drive score as a composable classifier input.
*/
func (flow *Flow) DriveReading() *FlowReading {
	return newFlowReading(flow, func(outcome FlowOutcome) float64 {
		return outcome.Drive
	})
}

/*
BalanceReading returns stochastic-balance score as a composable classifier input.
*/
func (flow *Flow) BalanceReading() *FlowReading {
	return newFlowReading(flow, func(outcome FlowOutcome) float64 {
		return outcome.Balance
	})
}

/*
StarvationReading returns volume-starvation score as a composable classifier input.
*/
func (flow *Flow) StarvationReading() *FlowReading {
	return newFlowReading(flow, func(outcome FlowOutcome) float64 {
		return outcome.Starvation
	})
}

/*
FlowReading exposes one FlowOutcome field as a pipeline score source.
*/
type FlowReading struct {
	artifact *datura.Artifact
	flow     *Flow
	project  func(FlowOutcome) float64
}

func newFlowReading(
	flow *Flow,
	project func(FlowOutcome) float64,
) *FlowReading {
	return &FlowReading{
		artifact: datura.Acquire("flow-reading", datura.Artifact_Type_json),
		flow:     flow,
		project:  project,
	}
}

func (reading *FlowReading) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	return len(p), nil
}

func (reading *FlowReading) Read(p []byte) (int, error) {
	value := 0.0

	if reading.flow != nil && reading.project != nil {
		value = reading.project(reading.flow.outcome)
	}

	_ = reading.artifact.SetPayload(encodePayload(value))

	return reading.artifact.Read(p)
}

func (reading *FlowReading) Close() error {
	return nil
}

const flowCategoryStarvation = 4

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

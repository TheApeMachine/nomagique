package equation

import (
	"math"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/statistic"
)

/*
Flow classifies signed trade pressure against price response in a rolling window.
*/
type Flow struct{}

/*
FlowInput contains the float-only trade-flow inputs.
*/
type FlowInput struct {
	BuyNotional    float64
	SellNotional   float64
	TradeCount     int
	GrossFloor     float64
	MedianNotional float64
	Prices         []float64
}

/*
FlowOutput contains the float-only trade-flow scores.
*/
type FlowOutput struct {
	Value       float64
	Absorption  float64
	Drive       float64
	Balance     float64
	Starvation  float64
	Net         float64
	NetFraction float64
	Category    float64
}

/*
NewFlow returns a CVD flow calculator.
*/
func NewFlow() *Flow {
	return &Flow{}
}

/*
Measure calculates flow scores from floats without artifact transport.
*/
func (flow *Flow) Measure(input FlowInput) (FlowOutput, error) {
	gross := input.BuyNotional + input.SellNotional

	if gross <= 0 || input.TradeCount <= 0 || len(input.Prices) == 0 {
		return FlowOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"flow: invalid input",
			nil,
		))
	}

	net := input.BuyNotional - input.SellNotional
	netFraction := math.Abs(net) / gross

	if len(input.Prices) < 2 {
		return FlowOutput{
			Net:         net,
			NetFraction: netFraction,
		}, nil
	}

	firstPrice := input.Prices[0]
	lastPrice := input.Prices[len(input.Prices)-1]

	if firstPrice <= 0 || lastPrice <= 0 {
		return FlowOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"flow: invalid price input",
			nil,
		))
	}

	if input.MedianNotional <= 0 ||
		math.IsNaN(input.MedianNotional) ||
		math.IsInf(input.MedianNotional, 0) {
		return FlowOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"flow: median notional must be positive",
			nil,
		))
	}

	priceResponseBps := math.Abs(lastPrice/firstPrice-1) * basisPointsPerUnit
	flowPressure := gross / input.MedianNotional

	if flowPressure <= 0 || math.IsNaN(flowPressure) || math.IsInf(flowPressure, 0) {
		return FlowOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"flow: invalid pressure input",
			nil,
		))
	}

	impactEfficiency := priceResponseBps / flowPressure
	priceDrift := lastPrice - firstPrice
	flatThreshold := medianAbsoluteMove(input.Prices)
	driveThreshold := 1 / math.Sqrt(float64(input.TradeCount))

	highNet := netFraction >= driveThreshold
	flowAligned := (net > 0 && priceDrift > 0) || (net < 0 && priceDrift < 0)
	hiddenAbsorption := flowHiddenAbsorption(
		highNet, flowPressure, impactEfficiency, priceDrift, flatThreshold, input.TradeCount,
	)
	flatPrice := hiddenAbsorption || flowFlatPrice(priceDrift, flatThreshold, input.TradeCount)

	category := flowCategory(
		highNet,
		flowAligned,
		flatPrice,
		gross,
		input.GrossFloor,
		input.TradeCount,
		hiddenAbsorption,
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
		if input.GrossFloor > 0 && gross < input.GrossFloor {
			starvation = math.Max(0, 1-gross/input.GrossFloor)
		}

		if input.TradeCount < 3 && !highNet {
			starvation = math.Max(starvation, 1-float64(input.TradeCount)/3)
			absorption = 0
			drive = 0
			balance = 0
		}
	}

	strength := math.Max(absorption, math.Max(drive, math.Max(balance, starvation)))

	return FlowOutput{
		Value:       strength,
		Absorption:  absorption,
		Drive:       drive,
		Balance:     balance,
		Starvation:  starvation,
		Net:         net,
		NetFraction: netFraction,
		Category:    float64(category),
	}, nil
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

	median, ok := statistic.MedianOf(absoluteMoves)
	if !ok {
		return 0
	}

	return median
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

package algorithm

import (
	"fmt"
	"math"

	"github.com/theapemachine/errnie"
)

const (
	pearlDefaultMinHistory = 5
	pearlSampleNodeCount   = 4
	pearlNodeMacro         = 0
	pearlNodeLiquidity     = 1
	pearlNodeFlow          = 2
	pearlNodeTarget        = 3
)

/*
PearlSample turns ticker, book, and trade rows into aligned causal rows.
*/
type PearlSample struct {
	minHistory int
	history    int
	windows    map[string]*pearlWindow
}

/*
PearlTickerInput is one ticker observation.
*/
type PearlTickerInput struct {
	Symbol    string
	Last      float64
	ChangePct float64
	Bid       float64
	Ask       float64
	BidQty    float64
	AskQty    float64
}

/*
PearlBookInput is one book observation.
*/
type PearlBookInput struct {
	Symbol string
	Bids   []BookLevel
	Asks   []BookLevel
}

/*
PearlTradeInput is one trade observation.
*/
type PearlTradeInput struct {
	Symbol   string
	Price    float64
	Quantity float64
	Side     string
}

/*
PearlSampleOutput is the current causal row and retained table.
*/
type PearlSampleOutput struct {
	Symbol string
	Row    []float64
	Rows   [][]float64
}

type pearlWindow struct {
	rows          [][]float64
	bids          map[float64]float64
	asks          map[float64]float64
	lastMacro     float64
	lastLiquidity float64
	lastFlow      float64
	lastPrice     float64
}

/*
NewPearlSample returns a direct Pearl sampler.
*/
func NewPearlSample(configs ...PearlConfig) *PearlSample {
	config := PearlConfig{}

	if len(configs) > 0 {
		config = configs[0]
	}

	minHistory := config.MinHistory

	if minHistory <= 0 {
		minHistory = pearlDefaultMinHistory
	}

	history := config.History

	if history < minHistory {
		history = minHistory
	}

	return &PearlSample{
		minHistory: minHistory,
		history:    history,
		windows:    map[string]*pearlWindow{},
	}
}

/*
MeasureTicker observes one ticker row.
*/
func (pearlSample *PearlSample) MeasureTicker(
	input PearlTickerInput,
) (PearlSampleOutput, bool, error) {
	if input.Symbol == "" || input.Last <= 0 {
		return PearlSampleOutput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"pearl-sample: ticker requires symbol and last",
			nil,
		))
	}

	window := pearlSample.window(input.Symbol)
	liquidity := pearlSample.liquidityStress(input.Bid, input.Ask, input.BidQty, input.AskQty)

	if liquidity > 0 {
		window.lastLiquidity = liquidity
	}

	window.lastMacro = input.ChangePct / 100
	row := []float64{
		window.lastMacro,
		window.lastLiquidity,
		window.lastFlow,
		window.velocity(input.Last),
	}

	return pearlSample.append(input.Symbol, row, window), len(window.rows) >= pearlSample.minHistory, nil
}

/*
MeasureBook observes one book row.
*/
func (pearlSample *PearlSample) MeasureBook(
	input PearlBookInput,
) (PearlSampleOutput, bool, error) {
	if input.Symbol == "" {
		return PearlSampleOutput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"pearl-sample: book requires symbol",
			nil,
		))
	}

	window := pearlSample.window(input.Symbol)
	pearlSample.applyLevels(window.bids, input.Bids)
	pearlSample.applyLevels(window.asks, input.Asks)

	bid, bidQty := pearlSample.bestBid(window.bids)
	ask, askQty := pearlSample.bestAsk(window.asks)
	liquidity := pearlSample.liquidityStress(bid, ask, bidQty, askQty)

	if liquidity <= 0 {
		return PearlSampleOutput{}, false, nil
	}

	window.lastLiquidity = liquidity
	row := []float64{
		window.lastMacro,
		window.lastLiquidity,
		window.lastFlow,
		window.velocity((bid + ask) / 2),
	}

	return pearlSample.append(input.Symbol, row, window), len(window.rows) >= pearlSample.minHistory, nil
}

/*
MeasureTrade observes one trade row.
*/
func (pearlSample *PearlSample) MeasureTrade(
	input PearlTradeInput,
) (PearlSampleOutput, bool, error) {
	if input.Symbol == "" || input.Price <= 0 || input.Quantity <= 0 {
		return PearlSampleOutput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"pearl-sample: trade requires symbol, price, and quantity",
			nil,
		))
	}

	if input.Side != "buy" && input.Side != "sell" {
		return PearlSampleOutput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			fmt.Sprintf("pearl-sample: unknown side %q", input.Side),
			nil,
		))
	}

	window := pearlSample.window(input.Symbol)
	flow := input.Quantity

	if input.Side == "sell" {
		flow = -input.Quantity
	}

	window.lastFlow = flow
	row := []float64{
		window.lastMacro,
		window.lastLiquidity,
		window.lastFlow,
		window.velocity(input.Price),
	}

	return pearlSample.append(input.Symbol, row, window), len(window.rows) >= pearlSample.minHistory, nil
}

func (pearlSample *PearlSample) append(
	symbol string,
	row []float64,
	window *pearlWindow,
) PearlSampleOutput {
	window.rows = append(window.rows, append([]float64(nil), row...))

	if len(window.rows) > pearlSample.history {
		window.rows = window.rows[len(window.rows)-pearlSample.history:]
	}

	rows := make([][]float64, 0, len(window.rows))

	for _, retained := range window.rows {
		rows = append(rows, append([]float64(nil), retained...))
	}

	return PearlSampleOutput{
		Symbol: symbol,
		Row:    append([]float64(nil), row...),
		Rows:   rows,
	}
}

func (pearlSample *PearlSample) window(symbol string) *pearlWindow {
	window, ok := pearlSample.windows[symbol]

	if ok {
		return window
	}

	window = &pearlWindow{
		bids: map[float64]float64{},
		asks: map[float64]float64{},
	}
	pearlSample.windows[symbol] = window

	return window
}

func (pearlSample *PearlSample) applyLevels(book map[float64]float64, levels []BookLevel) {
	for _, level := range levels {
		if level.Price <= 0 {
			continue
		}

		if level.Quantity <= 0 {
			delete(book, level.Price)
			continue
		}

		book[level.Price] = level.Quantity
	}
}

func (pearlSample *PearlSample) bestBid(book map[float64]float64) (float64, float64) {
	price := 0.0
	quantity := 0.0

	for levelPrice, levelQuantity := range book {
		if levelPrice <= price {
			continue
		}

		price = levelPrice
		quantity = levelQuantity
	}

	return price, quantity
}

func (pearlSample *PearlSample) bestAsk(book map[float64]float64) (float64, float64) {
	price := 0.0
	quantity := 0.0

	for levelPrice, levelQuantity := range book {
		if price > 0 && levelPrice >= price {
			continue
		}

		price = levelPrice
		quantity = levelQuantity
	}

	return price, quantity
}

func (pearlSample *PearlSample) liquidityStress(
	bid float64,
	ask float64,
	bidQty float64,
	askQty float64,
) float64 {
	if bid <= 0 || ask <= 0 || ask <= bid {
		return 0
	}

	depth := bidQty + askQty

	if depth <= 0 {
		return 0
	}

	stress := (ask - bid) / depth

	if math.IsNaN(stress) || math.IsInf(stress, 0) {
		return 0
	}

	return stress
}

func (window *pearlWindow) velocity(price float64) float64 {
	velocity := 0.0

	if window.lastPrice > 0 && price > 0 {
		velocity = math.Log(price / window.lastPrice)
	}

	if price > 0 {
		window.lastPrice = price
	}

	return velocity
}

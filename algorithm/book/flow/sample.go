package flow

import (
	"math"
	"sync"
	"time"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/utils"
)

/*
Sample accumulates book and trade frames into the feature batch expects.
Each symbol is owned by one serial window so concurrent first-use cannot fork
state, and book updates commit both sides atomically before features run.
*/
type Sample struct {
	mu              sync.Mutex
	windows         map[string]*Window
	historyCapacity int
}

/*
BookLevel is one price/quantity level.
*/
type BookLevel struct {
	Price    float64
	Ticks    int64
	Quantity float64
}

/*
BookInput is one book update for a symbol.
*/
type BookInput struct {
	Symbol   string
	TickSize float64
	Bids     []BookLevel
	Asks     []BookLevel
}

/*
TradeSide identifies aggressive trade direction.
*/
type TradeSide string

const (
	TradeBuy  TradeSide = "buy"
	TradeSell TradeSide = "sell"
)

/*
Validate reports whether the trade side is executable.
*/
func (side TradeSide) Validate() error {
	if side == TradeBuy || side == TradeSell {
		return nil
	}

	return errnie.Error(errnie.Err(
		errnie.Validation,
		"-sample: trade side must be buy or sell",
		nil,
	))
}

/*
TradeInput is one trade update for a symbol.
*/
type TradeInput struct {
	Symbol   string
	Price    float64
	Quantity float64
	Side     TradeSide
	At       time.Time
}

/*
Window retains one symbol's book and rolling imbalance history.
*/
type Window struct {
	book           *Book
	weightedHist   []float64
	level1Hist     []float64
	flatHist       []float64
	tradePressure  float64
	tradeAt        time.Time
	tradeGapSum    float64
	tradeGaps      int
	lastMid        float64
	lastSpread     float64
	touchDepth     float64
	flatOK         bool
	touchCancelBid float64
	touchCancelAsk float64
	frameAddBid    float64
	frameAddAsk    float64
	observations   int
}

/*
NewSample returns a book/trade sampler whose retained observation capacity is
supplied by the composition root rather than hidden in the classifier.
*/
func NewSample(historyCapacity int) (*Sample, error) {
	if historyCapacity <= 0 {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"-sample: positive history capacity required",
			nil,
		))
	}

	return &Sample{
		windows:         map[string]*Window{},
		historyCapacity: historyCapacity,
	}, nil
}

/*
MeasureBook observes one book update and returns book-flow input, whether the
book is valid enough to score, and a confidence maturity for that reading.
*/
func (sample *Sample) MeasureBook(
	input BookInput,
) (equation.BookflowInput, bool, float64, error) {
	sample.mu.Lock()
	defer sample.mu.Unlock()

	if input.Symbol == "" {
		return equation.BookflowInput{}, false, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"-sample: symbol required",
			nil,
		))
	}

	window := sample.window(input.Symbol)

	if err := sample.ingestBook(input, window); err != nil {
		return equation.BookflowInput{}, false, 0, err
	}

	input2, ready, err := sample.features(window)

	return input2, ready, sample.maturity(window), err
}

/*
MeasureTrade observes one trade update and returns book-flow input, whether
the book is valid enough to score, and a confidence maturity for that reading.
*/
func (sample *Sample) MeasureTrade(
	input TradeInput,
) (equation.BookflowInput, bool, float64, error) {
	sample.mu.Lock()
	defer sample.mu.Unlock()

	if input.Symbol == "" {
		return equation.BookflowInput{}, false, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"-sample: symbol required",
			nil,
		))
	}

	if input.Price <= 0 || input.Quantity <= 0 {
		return equation.BookflowInput{}, false, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"-sample: trade price and quantity required",
			nil,
		))
	}

	if err := input.Side.Validate(); err != nil {
		return equation.BookflowInput{}, false, 0, err
	}

	if input.At.IsZero() {
		return equation.BookflowInput{}, false, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"-sample: trade timestamp required",
			nil,
		))
	}

	window := sample.window(input.Symbol)

	if err := sample.ingestTrade(input, window); err != nil {
		return equation.BookflowInput{}, false, 0, err
	}

	output, ready, err := sample.features(window)

	return output, ready, sample.maturity(window), err
}

/*
maturity reports a monotonically increasing, asymptotic confidence in the
window's history-derived thresholds as more book observations accumulate.
*/
func (sample *Sample) maturity(window *Window) float64 {
	observations := float64(window.observations)

	return observations / (observations + 1)
}

func (sample *Sample) window(symbol string) *Window {
	existing, ok := sample.windows[symbol]

	if ok {
		return existing
	}

	window := &Window{
		book: NewBook(),
	}
	sample.windows[symbol] = window

	return window
}

func (sample *Sample) ingestBook(
	input BookInput,
	window *Window,
) error {
	window.touchCancelBid = 0
	window.touchCancelAsk = 0
	window.frameAddBid = 0
	window.frameAddAsk = 0

	if err := window.book.Configure(input); err != nil {
		return err
	}

	bidFrame, askFrame, err := window.book.ApplyBook(input.Bids, input.Asks)

	if err != nil {
		return err
	}

	window.touchCancelBid = bidFrame.touchCancel
	window.touchCancelAsk = askFrame.touchCancel
	window.frameAddBid = bidFrame.frameAdd
	window.frameAddAsk = askFrame.frameAdd

	if !window.book.TwoSided() {
		return nil
	}

	mid := window.book.Mid()
	spread := window.book.Spread()
	decayRate := DecayRate(mid, spread)
	touchDepth := window.book.TouchDepth()
	toxicBid := ToxicPenalty(window.touchCancelBid, window.frameAddBid, touchDepth)
	toxicAsk := ToxicPenalty(window.touchCancelAsk, window.frameAddAsk, touchDepth)
	weighted := window.book.Imbalance(mid, decayRate, false, 0, toxicBid, toxicAsk)
	level1 := window.book.Imbalance(mid, decayRate, true, 0, toxicBid, toxicAsk)
	flatDepth, err := window.book.FlatDepth()

	if err != nil {
		return errnie.Error(errnie.Err(
			errnie.Validation,
			"-sample: flat depth resolution failed",
			err,
		))
	}

	flat := window.book.Imbalance(mid, decayRate, false, flatDepth, toxicBid, toxicAsk)
	window.lastMid = mid
	window.lastSpread = spread
	window.touchDepth = touchDepth
	window.flatOK = flatDepth > 0
	window.observations++
	window.weightedHist = utils.AppendRingFloat(
		window.weightedHist,
		weighted,
		sample.historyCapacity,
	)
	window.level1Hist = utils.AppendRingFloat(
		window.level1Hist,
		level1,
		sample.historyCapacity,
	)
	window.flatHist = utils.AppendRingFloat(
		window.flatHist,
		flat,
		sample.historyCapacity,
	)

	return nil
}

func (sample *Sample) ingestTrade(
	input TradeInput,
	window *Window,
) error {
	notional := input.Price * input.Quantity
	signedNotional := notional

	if input.Side == TradeSell {
		signedNotional = -notional
	}

	if window.tradeAt.IsZero() {
		window.tradePressure = signedNotional
		window.tradeAt = input.At

		return nil
	}

	if input.At.Before(window.tradeAt) {
		return errnie.Error(errnie.Err(
			errnie.Validation,
			"-sample: trade timestamp must not regress",
			nil,
		))
	}

	elapsed := input.At.Sub(window.tradeAt).Seconds()

	if elapsed < 0 {
		return errnie.Error(errnie.Err(
			errnie.Validation,
			"-sample: trade timestamp must not regress",
			nil,
		))
	}

	window.tradeGaps++
	window.tradeGapSum += elapsed
	halfLife := window.tradeGapSum / float64(window.tradeGaps)

	if halfLife <= 0 {
		window.tradePressure = signedNotional
		window.tradeAt = input.At

		return nil
	}

	alpha := 1 - math.Exp(-math.Ln2*elapsed/halfLife)
	window.tradePressure += alpha * (signedNotional - window.tradePressure)
	window.tradeAt = input.At

	return nil
}

func (sample *Sample) features(
	window *Window,
) (equation.BookflowInput, bool, error) {
	if !window.book.TwoSided() || len(window.weightedHist) == 0 {
		return equation.BookflowInput{
			Mid:           window.lastMid,
			Spread:        window.lastSpread,
			TouchDepth:    window.touchDepth,
			TradePressure: window.tradePressure,
		}, false, nil
	}

	weighted := window.weightedHist[len(window.weightedHist)-1]
	level1 := window.level1Hist[len(window.level1Hist)-1]
	flat := window.flatHist[len(window.flatHist)-1]

	return equation.BookflowInput{
		Weighted:        weighted,
		Level1:          level1,
		Flat:            flat,
		FlatOK:          window.flatOK,
		Mid:             window.lastMid,
		Spread:          window.lastSpread,
		TouchDepth:      window.touchDepth,
		TradePressure:   window.tradePressure,
		WeightedHistory: window.weightedHist,
		Level1History:   window.level1Hist,
		FlatHistory:     window.flatHist,
	}, true, nil
}

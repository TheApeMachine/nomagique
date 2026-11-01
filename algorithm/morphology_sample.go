package algorithm

import (
	"math"
	"sync"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/algorithm/book/flow"
	"github.com/theapemachine/nomagique/equation"
)

/*
MorphologySample owns per-symbol book continuity for morphology scoring.
*/
type MorphologySample struct {
	windows *sync.Map
}

type morphologyWindow struct {
	mu              sync.Mutex
	prevBook        []equation.MorphologyPoint
	askTop3Baseline float64
	hasPrev         bool
}

/*
NewMorphologySample returns a book-shape sampler.
*/
func NewMorphologySample() *MorphologySample {
	return &MorphologySample{windows: &sync.Map{}}
}

/*
MeasureBook observes one book snapshot and emits morphology input.
*/
func (sample *MorphologySample) MeasureBook(
	input flow.BookInput,
) (equation.MorphologyInput, bool, error) {
	if input.Symbol == "" {
		return equation.MorphologyInput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"morphology-sample: symbol required",
			nil,
		))
	}

	if len(input.Bids) == 0 || len(input.Asks) == 0 {
		return equation.MorphologyInput{}, false, nil
	}

	bids := make([]equation.MorphologyLevel, 0, len(input.Bids))
	asks := make([]equation.MorphologyLevel, 0, len(input.Asks))

	for _, level := range input.Bids {
		if level.Price > 0 && level.Quantity > 0 {
			bids = append(bids, equation.MorphologyLevel{
				Price: level.Price, Quantity: level.Quantity,
			})
		}
	}
	for _, level := range input.Asks {
		if level.Price > 0 && level.Quantity > 0 {
			asks = append(asks, equation.MorphologyLevel{
				Price: level.Price, Quantity: level.Quantity,
			})
		}
	}

	if len(bids) == 0 || len(asks) == 0 {
		return equation.MorphologyInput{}, false, nil
	}

	bestBid, bestAsk := bids[0].Price, asks[0].Price
	for _, level := range bids {
		if level.Price > bestBid {
			bestBid = level.Price
		}
	}
	for _, level := range asks {
		if level.Price < bestAsk {
			bestAsk = level.Price
		}
	}
	if bestAsk <= bestBid {
		return equation.MorphologyInput{}, false, nil
	}

	mid := (bestBid + bestAsk) / 2
	spread := bestAsk - bestBid
	askTop3 := askTopNotional(asks)

	curve, err := equation.MorphBookCurve(bids, asks, mid, spread)
	if err != nil {
		return equation.MorphologyInput{}, false, err
	}

	window := sample.window(input.Symbol)
	window.mu.Lock()
	defer window.mu.Unlock()

	morphChange := 0.0
	hasMorph := window.hasPrev

	if hasMorph {
		_, morphChange = equation.MorphWalk(window.prevBook, curve)
	}

	if window.askTop3Baseline <= 0 {
		window.askTop3Baseline = askTop3
	} else {
		window.askTop3Baseline = 0.94*window.askTop3Baseline + 0.06*askTop3
	}

	vacuum := 0.0

	if window.askTop3Baseline > 0 && askTop3 > 0 {
		vacuum = math.Max(0, 1-askTop3/window.askTop3Baseline)
	}

	window.prevBook = curve
	window.hasPrev = true

	return equation.MorphologyInput{
		Bids:        bids,
		Asks:        asks,
		AskVacuum:   vacuum,
		MorphChange: morphChange,
		HasMorph:    hasMorph,
	}, true, nil
}

func askTopNotional(asks []equation.MorphologyLevel) float64 {
	top3 := 0.0
	for index := 0; index < len(asks) && index < 3; index++ {
		top3 += asks[index].Price * asks[index].Quantity
	}
	return top3
}

func (sample *MorphologySample) window(symbol string) *morphologyWindow {
	if existing, ok := sample.windows.Load(symbol); ok {
		return existing.(*morphologyWindow)
	}

	created := &morphologyWindow{}
	actual, _ := sample.windows.LoadOrStore(symbol, created)
	return actual.(*morphologyWindow)
}

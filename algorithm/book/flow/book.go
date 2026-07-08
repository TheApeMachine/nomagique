package flow

import (
	"math"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/statistic"
)

type Book struct {
	tickSize float64
	bids     *SideBook
	asks     *SideBook
}

type SideBook struct {
	side   byte
	levels map[int64]float64
	ticks  []int64
}

type Frame struct {
	touchCancel float64
	frameAdd    float64
}

func NewBook() *Book {
	return &Book{
		bids: NewSideBook(SideBid),
		asks: NewSideBook(SideAsk),
	}
}

func NewSideBook(side byte) *SideBook {
	return &SideBook{
		side:   side,
		levels: map[int64]float64{},
	}
}

func (book *Book) Configure(input BookInput) error {
	tickSize := input.TickSize

	if tickSize <= 0 || math.IsNaN(tickSize) || math.IsInf(tickSize, 0) {
		return errnie.Error(errnie.Err(
			errnie.Validation,
			"-sample: positive finite tick size required",
			nil,
		))
	}

	if book.tickSize == tickSize {
		return nil
	}

	if book.tickSize > 0 {
		return errnie.Error(errnie.Err(
			errnie.Validation,
			"-sample: tick size changed for active book",
			nil,
		))
	}

	book.tickSize = tickSize

	return nil
}

func (book *Book) ApplyLevels(
	levels []BookLevel,
	side byte,
) (Frame, error) {
	return book.side(side).Apply(levels, book.tickSize)
}

func (book *Book) Mid() float64 {
	bestBid := book.bids.Best(book.tickSize)
	bestAsk := book.asks.Best(book.tickSize)

	if bestBid <= 0 || bestAsk <= 0 {
		return 0
	}

	return (bestBid + bestAsk) / 2
}

func (book *Book) Spread() float64 {
	bestBid := book.bids.Best(book.tickSize)
	bestAsk := book.asks.Best(book.tickSize)

	if bestBid <= 0 || bestAsk <= 0 || bestAsk <= bestBid {
		return 0
	}

	return bestAsk - bestBid
}

func (book *Book) TouchDepth() float64 {
	return book.bids.TouchQty() + book.asks.TouchQty()
}

func (book *Book) SideDepth(side byte) float64 {
	return book.side(side).Depth()
}

func (book *Book) FlatDepth() (int, error) {
	levelCount := book.bids.Len() + book.asks.Len()

	if levelCount < 2 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"-sample: flat depth needs at least two levels",
			nil,
		))
	}

	_, longWindow, err := statistic.ResolveWindows(make([]float64, levelCount), 0, 0)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"-sample: flat depth window resolution failed",
			err,
		))
	}

	flatDepth := int(math.Ceil(math.Sqrt(float64(levelCount))))

	if flatDepth < 2 {
		flatDepth = 2
	}

	if flatDepth > longWindow {
		flatDepth = longWindow
	}

	return flatDepth, nil
}

func (book *Book) Imbalance(
	mid float64,
	decayRate float64,
	touchOnly bool,
	flatDepth int,
	toxicBid float64,
	toxicAsk float64,
) float64 {
	bidWeight := book.bids.SideWeight(mid, decayRate, touchOnly, flatDepth, book.tickSize)
	askWeight := book.asks.SideWeight(mid, decayRate, touchOnly, flatDepth, book.tickSize)

	if toxicBid > 0 {
		bidWeight *= 1 - toxicBid
	}

	if toxicAsk > 0 {
		askWeight *= 1 - toxicAsk
	}

	total := bidWeight + askWeight

	if total <= 0 {
		return 0
	}

	return (bidWeight - askWeight) / total
}

func (book *Book) side(side byte) *SideBook {
	if side == SideBid {
		return book.bids
	}

	return book.asks
}

func (sideBook *SideBook) Apply(
	levels []BookLevel,
	tickSize float64,
) (Frame, error) {
	frame := Frame{}

	for _, level := range levels {
		tick, err := LevelTick(level)

		if err != nil {
			return Frame{}, err
		}

		if math.IsNaN(level.Quantity) || math.IsInf(level.Quantity, 0) || level.Quantity < 0 {
			return Frame{}, errnie.Error(errnie.Err(
				errnie.Validation,
				"-sample: level quantity must be finite and non-negative",
				nil,
			))
		}

		previousQty := sideBook.levels[tick]
		touch := sideBook.isTouchTick(tick)

		if level.Quantity == 0 {
			delete(sideBook.levels, tick)

			if previousQty > 0 && touch {
				frame.touchCancel += previousQty
			}

			continue
		}

		delta := level.Quantity - previousQty
		sideBook.levels[tick] = level.Quantity

		if delta <= 0 {
			if touch {
				frame.touchCancel += -delta
			}

			continue
		}

		frame.frameAdd += delta
	}

	return frame, nil
}

func (sideBook *SideBook) Best(tickSize float64) float64 {
	tick, ok := sideBook.bestTick()

	if !ok {
		return 0
	}

	return TickPrice(tick, tickSize)
}

func (sideBook *SideBook) TouchQty() float64 {
	tick, ok := sideBook.bestTick()

	if !ok {
		return 0
	}

	return sideBook.levels[tick]
}

func (sideBook *SideBook) Depth() float64 {
	depth := 0.0

	for _, quantity := range sideBook.levels {
		depth += quantity
	}

	return depth
}

func (sideBook *SideBook) SideWeight(
	mid float64,
	decayRate float64,
	touchOnly bool,
	flatDepth int,
	tickSize float64,
) float64 {
	if touchOnly {
		return sideBook.TouchQty()
	}

	weight := 0.0
	remaining := flatDepth
	ticks := sideBook.sortedTicks(mid, tickSize)

	for _, tick := range ticks {
		if flatDepth > 0 {
			if remaining <= 0 {
				break
			}

			remaining--
		}

		price := TickPrice(tick, tickSize)
		distance := math.Abs(price-mid) / mid
		kernel := math.Exp(-decayRate * distance)
		weight += sideBook.levels[tick] * kernel
	}

	return weight
}

func (sideBook *SideBook) Len() int {
	return len(sideBook.levels)
}

func (sideBook *SideBook) bestTick() (int64, bool) {
	var bestTick int64
	ok := false

	for tick := range sideBook.levels {
		if !ok {
			bestTick = tick
			ok = true

			continue
		}

		if sideBook.side == SideBid && tick > bestTick {
			bestTick = tick
		}

		if sideBook.side == SideAsk && tick < bestTick {
			bestTick = tick
		}
	}

	return bestTick, ok
}

func (sideBook *SideBook) sortedTicks(mid float64, tickSize float64) []int64 {
	sideBook.ticks = sideBook.ticks[:0]

	for tick := range sideBook.levels {
		sideBook.ticks = append(sideBook.ticks, tick)
	}

	for left := 1; left < len(sideBook.ticks); left++ {
		cursor := sideBook.ticks[left]

		for index := left - 1; index >= 0; index-- {
			leftPrice := TickPrice(sideBook.ticks[index], tickSize)
			cursorPrice := TickPrice(cursor, tickSize)
			leftDistance := math.Abs(leftPrice - mid)
			cursorDistance := math.Abs(cursorPrice - mid)

			if leftDistance <= cursorDistance {
				break
			}

			sideBook.ticks[index+1] = sideBook.ticks[index]
			sideBook.ticks[index] = cursor
		}
	}

	return sideBook.ticks
}

func (sideBook *SideBook) isTouchTick(tick int64) bool {
	bestTick, ok := sideBook.bestTick()

	return ok && bestTick == tick
}

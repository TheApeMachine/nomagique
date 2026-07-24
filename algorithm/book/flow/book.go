package flow

import (
	"math"
	"math/big"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/statistic"
)

/*
Side identifies one book side. Values other than bid or ask are rejected before
any resting state is touched.
*/
type Side byte

const (
	SideBid Side = 'b'
	SideAsk Side = 'a'
)

/*
Validate reports whether the side is an executable book side.
*/
func (side Side) Validate() error {
	if side == SideBid || side == SideAsk {
		return nil
	}

	return errnie.Error(errnie.Err(
		errnie.Validation,
		"-sample: book side must be bid or ask",
		nil,
	))
}

/*
Book is a single-owner integer-tick order book with cached best and depth.
*/
type Book struct {
	tick *big.Rat
	bids *SideBook
	asks *SideBook
}

/*
SideBook retains one side's resting levels in tick order.
*/
type SideBook struct {
	side    Side
	levels  map[int64]float64
	ordered []int64
	best    int64
	hasBest bool
	depth   float64
}

/*
Frame accumulates touch-cancel and add volume for one atomic side update.
*/
type Frame struct {
	touchCancel float64
	frameAdd    float64
}

/*
NewBook returns an empty two-sided book.
*/
func NewBook() *Book {
	return &Book{
		bids: NewSideBook(SideBid),
		asks: NewSideBook(SideAsk),
	}
}

/*
NewSideBook returns one empty side ledger.
*/
func NewSideBook(side Side) *SideBook {
	return &SideBook{
		side:   side,
		levels: map[int64]float64{},
	}
}

/*
Configure locks the book onto one price lattice. Later calls with the same
economic tick are ignored; a true increment change clears resting levels and
adopts the new lattice so instrument refreshes cannot poison an active book.
*/
func (book *Book) Configure(input BookInput) error {
	tick, err := parseTickSize(input.TickSize)

	if err != nil {
		return err
	}

	if book.tick == nil {
		book.tick = tick

		return nil
	}

	if book.tick.Cmp(tick) == 0 {
		return nil
	}

	book.bids = NewSideBook(SideBid)
	book.asks = NewSideBook(SideAsk)
	book.tick = tick

	return nil
}

/*
TickSize returns the configured lattice as float64 for price projection.
*/
func (book *Book) TickSize() float64 {
	if book.tick == nil {
		return 0
	}

	value, _ := book.tick.Float64()

	return value
}

/*
ApplyLevels validates an entire side batch, then applies it atomically.
*/
func (book *Book) ApplyLevels(levels []BookLevel, side Side) (Frame, error) {
	if err := side.Validate(); err != nil {
		return Frame{}, err
	}

	return book.side(side).Apply(levels, book.TickSize())
}

/*
ApplyBook validates and applies both sides as one book transaction.
*/
func (book *Book) ApplyBook(bids, asks []BookLevel) (Frame, Frame, error) {
	bidOps, err := book.bids.prepare(bids)

	if err != nil {
		return Frame{}, Frame{}, err
	}

	askOps, err := book.asks.prepare(asks)

	if err != nil {
		return Frame{}, Frame{}, err
	}

	return book.bids.commit(bidOps), book.asks.commit(askOps), nil
}

/*
Mid returns the two-sided midpoint, or zero when either side is empty.
*/
func (book *Book) Mid() float64 {
	bestBid := book.bids.Best(book.TickSize())
	bestAsk := book.asks.Best(book.TickSize())

	if bestBid <= 0 || bestAsk <= 0 {
		return 0
	}

	return (bestBid + bestAsk) / 2
}

/*
Spread returns the two-sided spread, or zero when the book is not marketable.
*/
func (book *Book) Spread() float64 {
	bestBid := book.bids.Best(book.TickSize())
	bestAsk := book.asks.Best(book.TickSize())

	if bestBid <= 0 || bestAsk <= 0 || bestAsk <= bestBid {
		return 0
	}

	return bestAsk - bestBid
}

/*
TwoSided reports whether both sides have a positive marketable touch.
*/
func (book *Book) TwoSided() bool {
	return book.Mid() > 0 && book.Spread() > 0
}

/*
TouchDepth returns combined touch quantity.
*/
func (book *Book) TouchDepth() float64 {
	return book.bids.TouchQty() + book.asks.TouchQty()
}

/*
SideDepth returns total resting quantity on one validated side.
*/
func (book *Book) SideDepth(side Side) float64 {
	if err := side.Validate(); err != nil {
		return 0
	}

	return book.side(side).Depth()
}

/*
FlatDepth resolves how many near-touch levels participate in flat imbalance.
*/
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

/*
Imbalance computes signed depth pressure around the current midpoint.
*/
func (book *Book) Imbalance(
	mid float64,
	decayRate float64,
	touchOnly bool,
	flatDepth int,
	toxicBid float64,
	toxicAsk float64,
) float64 {
	bestBidTick, bidOK := book.bids.bestTick()
	bestAskTick, askOK := book.asks.bestTick()

	if mid <= 0 || !bidOK || !askOK || bestAskTick <= bestBidTick {
		return 0
	}

	midTick := (float64(bestBidTick) + float64(bestAskTick)) / 2
	bidWeight := book.bids.SideWeight(midTick, decayRate, touchOnly, flatDepth)
	askWeight := book.asks.SideWeight(midTick, decayRate, touchOnly, flatDepth)

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

func (book *Book) side(side Side) *SideBook {
	if side == SideBid {
		return book.bids
	}

	return book.asks
}


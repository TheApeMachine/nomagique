package market

import (
	"time"

	"github.com/shopspring/decimal"
)

/*
SideKind tags bid versus ask deltas broker BookManager applies from Level3.
*/
type SideKind string

/*
SideBid and SideAsk route venue L3 events into the correct book half.
*/
const (
	SideBid SideKind = "bid"
	SideAsk SideKind = "ask"
)

/*
Delta mutates one resting order or level on the book.
*/
type Delta struct {
	Direction SideKind
	ID        string
	Price     *decimal.Decimal
	Quantity  *decimal.Decimal
	Timestamp time.Time
}

/*
Book is a single-symbol L3 order book.
*/
type Book struct {
	Name           string    `json:"name,omitempty"`
	MaxDepth       int       `json:"maxDepth,omitempty"`
	NoBookCrossing bool      `json:"noBookCrossing,omitempty"`
	EnableMaxDepth bool      `json:"enableMaxDepth,omitempty"`
	Bids           *BookSide `json:"bids,omitempty"`
	Asks           *BookSide `json:"asks,omitempty"`
}

/*
NewBook builds an L3-ready book with depth caps; symm leaves crossing off so
checksum misses trigger resubscribe instead of silent heals.
*/
func NewBook() *Book {
	bids := NewBookSide()
	bids.Direction = SideBid
	asks := NewBookSide()
	asks.Direction = SideAsk

	return &Book{
		MaxDepth: 1e10,
		// Crossed L3 frames are venue desync, not something to heal by
		// deleting levels. Callers verify checksum and resubscribe instead.
		NoBookCrossing: false,
		EnableMaxDepth: true,
		Bids:           bids,
		Asks:           asks,
	}
}

/*
Midpoint feeds signals and oracle spread gates when both sides have liquidity.
*/
func (book *Book) Midpoint() *decimal.Decimal {
	bid, ask := book.BestBid(), book.BestAsk()

	switch {
	case bid != nil && ask != nil:
		mid := bid.Price.Add(*ask.Price).Mul(decimal.NewFromFloat(0.5))
		return &mid
	case bid != nil:
		return bid.Price
	case ask != nil:
		return ask.Price
	default:
		zero := decimal.Zero
		return &zero
	}
}

/*
Spread is touch-to-touch percent spread for desk tight_touch / blown_spread oracles.
*/
func (book *Book) Spread() *decimal.Decimal {
	bid, ask := book.BestBid(), book.BestAsk()

	if bid == nil || ask == nil {
		zero := decimal.Zero
		return &zero
	}

	spread := ask.Price.Sub(*bid.Price).Div(*ask.Price).Mul(decimal.NewFromInt(100))
	return &spread
}

/*
Update applies one L3 delta then optional cross/depth enforcement after broker ingest.
*/
func (book *Book) Update(delta *Delta) {
	switch delta.Direction {
	case SideAsk:
		book.Asks.apply(delta)
	case SideBid:
		book.Bids.apply(delta)
	}

	if book.NoBookCrossing {
		book.EnforceOrder()
	}

	if book.EnableMaxDepth {
		book.EnforceDepth()
	}
}

/*
BestBid is top-of-book bid for TouchBid sizing and L2/L3 checksum walks.
*/
func (book *Book) BestBid() *Level { return book.Bids.High }

/*
BestAsk is top-of-book ask for TouchAsk and spread oracles.
*/
func (book *Book) BestAsk() *Level { return book.Asks.Low }

/*
WorstAsk is the deepest ask level depth enforcement may delete.
*/
func (book *Book) WorstAsk() *Level { return book.Asks.High }

/*
WorstBid is the deepest bid level depth enforcement may delete.
*/
func (book *Book) WorstBid() *Level { return book.Bids.Low }

/*
EnforceOrder heals crossed books when NoBookCrossing is enabled (paper sim paths).
*/
func (book *Book) EnforceOrder() {
	zero := decimal.Zero

	for bid, ask := book.BestBid(), book.BestAsk(); bid != nil && ask != nil && bid.Price.Cmp(*ask.Price) >= 0; bid, ask = book.BestBid(), book.BestAsk() {
		if bid.Timestamp.After(ask.Timestamp) {
			book.Update(&Delta{
				Direction: SideAsk,
				Price:     ask.Price,
				Quantity:  &zero,
				Timestamp: time.Now(),
			})
			continue
		}

		book.Update(&Delta{
			Direction: SideBid,
			Price:     bid.Price,
			Quantity:  &zero,
			Timestamp: time.Now(),
		})
	}
}

/*
EnforceDepth trims far levels so L3 replay and signals stay within MaxDepth.
*/
func (book *Book) EnforceDepth() {
	zero := decimal.Zero

	for len(book.Bids.Levels) > book.MaxDepth {
		worst := book.WorstBid()
		book.Update(&Delta{
			Direction: SideBid,
			Price:     worst.Price,
			Quantity:  &zero,
			Timestamp: time.Now(),
		})
	}

	for len(book.Asks.Levels) > book.MaxDepth {
		worst := book.WorstAsk()
		book.Update(&Delta{
			Direction: SideAsk,
			Price:     worst.Price,
			Quantity:  &zero,
			Timestamp: time.Now(),
		})
	}
}

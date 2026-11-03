package market

import (
	"time"

	"github.com/shopspring/decimal"
)

type SideKind string

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

func NewBook() *Book {
	bids := NewBookSide()
	bids.Direction = SideBid
	asks := NewBookSide()
	asks.Direction = SideAsk

	return &Book{
		MaxDepth:       1e10,
		NoBookCrossing: true,
		EnableMaxDepth: true,
		Bids:           bids,
		Asks:           asks,
	}
}

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

func (book *Book) Spread() *decimal.Decimal {
	bid, ask := book.BestBid(), book.BestAsk()

	if bid == nil || ask == nil {
		zero := decimal.Zero
		return &zero
	}

	spread := ask.Price.Sub(*bid.Price).Div(*ask.Price).Mul(decimal.NewFromInt(100))
	return &spread
}

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

func (book *Book) BestBid() *Level  { return book.Bids.High }
func (book *Book) BestAsk() *Level  { return book.Asks.Low }
func (book *Book) WorstAsk() *Level { return book.Asks.High }
func (book *Book) WorstBid() *Level { return book.Bids.Low }

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

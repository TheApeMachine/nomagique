package market

import "github.com/shopspring/decimal"

/*
Update is a minimal price tick some feeds emit before full Trade frames land.
*/
type Update struct {
	Symbol string
	Price  *decimal.Decimal
}

/*
NewUpdate constructs a single-symbol mark for lightweight ingress paths.
*/
func NewUpdate(symbol string, price *decimal.Decimal) *Update {
	return &Update{
		Symbol: symbol,
		Price:  price,
	}
}

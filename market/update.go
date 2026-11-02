package market

import "github.com/shopspring/decimal"

type Update struct {
	Symbol string
	Price  *decimal.Decimal
}

func NewUpdate(symbol string, price *decimal.Decimal) *Update {
	return &Update{
		Symbol: symbol,
		Price:  price,
	}
}

package market

import (
	"time"

	"github.com/shopspring/decimal"
)

type Side string
type OrderType string

const (
	SideBuy  Side = "buy"
	SideSell Side = "sell"

	OrderTypeMarket OrderType = "market"
	OrderTypeLimit  OrderType = "limit"
)

func (side Side) String() string {
	return string(side)
}

func (orderType OrderType) String() string {
	return string(orderType)
}

type TradeData struct {
	Symbol    string          `json:"symbol"`
	Side      Side            `json:"side"`
	Price     decimal.Decimal `json:"price"`
	Qty       decimal.Decimal `json:"qty"`
	OrderType OrderType       `json:"ord_type"`
	TradeID   int64           `json:"trade_id"`
	Timestamp time.Time       `json:"timestamp"`
}

type Trade struct {
	Channel string       `json:"channel"`
	Type    string       `json:"type"`
	Data    []*TradeData `json:"data"`
}

func NewTrade(
	channel string,
	tradeType string,
	data []*TradeData,
) *Trade {
	return &Trade{
		Channel: channel,
		Type:    tradeType,
		Data:    data,
	}
}

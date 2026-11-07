package market

import (
	"encoding/json"
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
	Channel  string       `json:"channel"`
	Type     string       `json:"type"`
	Data     []*TradeData `json:"data"`
	onChange func(MarketType)
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

func (trade *Trade) OnChange(fn func(MarketType)) {
	trade.onChange = fn
}

func (trade *Trade) Request() map[string]any {
	return map[string]any{}
}

func (trade *Trade) ToMarket(other MarketType) MarketType {
	dst, ok := other.(*Trade)

	if !ok || dst == nil || trade == nil {
		return other
	}

	onChange := dst.onChange
	dst.Channel = trade.Channel
	dst.Type = trade.Type
	dst.Data = append([]*TradeData(nil), trade.Data...)
	dst.onChange = onChange

	return dst
}

func (trade *Trade) FromMarket(other MarketType) MarketType {
	src, ok := other.(*Trade)

	if !ok || src == nil || trade == nil {
		return trade
	}

	return src.ToMarket(trade)
}

func (trade *Trade) MarshalJSON() ([]byte, error) {
	type alias Trade
	return json.Marshal((*alias)(trade))
}

func (trade *Trade) UnmarshalJSON(data []byte) error {
	type alias Trade
	var decoded alias

	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	onChange := trade.onChange
	*trade = Trade(decoded)
	trade.onChange = onChange

	if trade.onChange != nil {
		trade.onChange(trade)
	}

	return nil
}

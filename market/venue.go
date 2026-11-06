package market

import "encoding/json"

type MarketType interface {
	json.Marshaler
	json.Unmarshaler
	Request() map[string]any
	ToMarket(MarketType) MarketType
	FromMarket(MarketType) MarketType
	OnChange(func(MarketType))
}

type Venue interface {
	Read([]byte) MarketType
	Balance() MarketType
	TradeBalance() MarketType
	Level3() MarketType
	Trade() MarketType
	Execution() MarketType
	Book() MarketType
	Ticker() MarketType
}

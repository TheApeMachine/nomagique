package market

import "encoding/json"

/*
MarketType is the frame contract venue.Read returns; desk tiers merge via ToMarket.
Decode stays pure — callers act on the returned value, not UnmarshalJSON side effects.
*/
type MarketType interface {
	json.Marshaler
	json.Unmarshaler
	Request() map[string]any
	ToMarket(MarketType) MarketType
	FromMarket(MarketType) MarketType
}

/*
Venue exposes typed channels (Level3, Trade, Book) without symm importing Kraken wire details.
*/
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

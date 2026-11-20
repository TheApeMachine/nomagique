package market

import (
	"encoding/json"

	"github.com/shopspring/decimal"
)

/*
TradeBalance is venue-agnostic equity / margin state from a trade-balance poll.
*/
type TradeBalance struct {
	Equity        *decimal.Decimal `json:"equity"`
	UnrealizedPnL *decimal.Decimal `json:"unrealized_pnl"`
	AvailableCash *decimal.Decimal `json:"available_cash"`
	FreeMargin    *decimal.Decimal `json:"free_margin"`
}

/*
NewTradeBalance decodes margin poll fields tiers use for risk and Enter gates.
*/
func NewTradeBalance(data map[string]any) *TradeBalance {
	out := &TradeBalance{}

	if value, ok := data["equity"].(*decimal.Decimal); ok {
		out.Equity = value
	}

	if value, ok := data["unrealized_pnl"].(*decimal.Decimal); ok {
		out.UnrealizedPnL = value
	}

	if value, ok := data["available_cash"].(*decimal.Decimal); ok {
		out.AvailableCash = value
	}

	if value, ok := data["free_margin"].(*decimal.Decimal); ok {
		out.FreeMargin = value
	}

	return out
}


/*
Request is empty: trade balance arrives on scheduled venue polls.
*/
func (balance *TradeBalance) Request() map[string]any {
	return map[string]any{}
}

/*
ToMarket merges margin snapshot into tier state for stoploss and sizing checks.
*/
func (balance *TradeBalance) ToMarket(other MarketType) MarketType {
	dst, ok := other.(*TradeBalance)

	if !ok || dst == nil || balance == nil {
		return other
	}

	dst.Equity = balance.Equity
	dst.UnrealizedPnL = balance.UnrealizedPnL
	dst.AvailableCash = balance.AvailableCash
	dst.FreeMargin = balance.FreeMargin

	return dst
}

/*
FromMarket copies peer margin into this slot for paper tier mirroring.
*/
func (balance *TradeBalance) FromMarket(other MarketType) MarketType {
	src, ok := other.(*TradeBalance)

	if !ok || src == nil || balance == nil {
		return balance
	}

	return src.ToMarket(balance)
}

/*
MarshalJSON embeds margin on ticks uploaded with envelope batches.
*/
func (balance *TradeBalance) MarshalJSON() ([]byte, error) {
	type alias TradeBalance
	return json.Marshal((*alias)(balance))
}

/*
UnmarshalJSON restores margin from store replay.
*/
func (balance *TradeBalance) UnmarshalJSON(data []byte) error {
	type alias TradeBalance
	var decoded alias

	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	*balance = TradeBalance(decoded)


	return nil
}

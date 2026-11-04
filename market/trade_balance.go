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
	onChange      func(MarketType)
}

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

func (balance *TradeBalance) OnChange(fn func(MarketType)) {
	balance.onChange = fn
}

func (balance *TradeBalance) Request() map[string]any {
	return map[string]any{}
}

func (balance *TradeBalance) ToMarket(other MarketType) MarketType {
	dst, ok := other.(*TradeBalance)

	if !ok || dst == nil || balance == nil {
		return other
	}

	onChange := dst.onChange
	dst.Equity = balance.Equity
	dst.UnrealizedPnL = balance.UnrealizedPnL
	dst.AvailableCash = balance.AvailableCash
	dst.FreeMargin = balance.FreeMargin
	dst.onChange = onChange

	return dst
}

func (balance *TradeBalance) FromMarket(other MarketType) MarketType {
	src, ok := other.(*TradeBalance)

	if !ok || src == nil || balance == nil {
		return balance
	}

	return src.ToMarket(balance)
}

func (balance *TradeBalance) MarshalJSON() ([]byte, error) {
	type alias TradeBalance
	return json.Marshal((*alias)(balance))
}

func (balance *TradeBalance) UnmarshalJSON(data []byte) error {
	type alias TradeBalance
	var decoded alias

	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	onChange := balance.onChange
	*balance = TradeBalance(decoded)
	balance.onChange = onChange

	if balance.onChange != nil {
		balance.onChange(balance)
	}

	return nil
}

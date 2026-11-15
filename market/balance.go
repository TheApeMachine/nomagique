package market

import (
	"encoding/json"

	"github.com/shopspring/decimal"
)

/*
Balance is spendable cash plus account equity. Cash comes from Balance REST;
equity and unrealized come from TradeBalance REST.
*/
type Balance struct {
	Quote      string           `json:"quote"`
	Assets     []*Asset         `json:"assets"`
	Equity     *decimal.Decimal `json:"equity,omitempty"`
	Unrealized *decimal.Decimal `json:"unrealized,omitempty"`
}

/*
NewBalance creates a new balance book with the given quote currency.
*/
func NewBalance(data map[string]any) *Balance {
	balance := &Balance{
		Quote:  data["quote"].(string),
		Assets: data["assets"].([]*Asset),
	}

	if value, ok := data["equity"].(*decimal.Decimal); ok {
		balance.Equity = value
	}

	if value, ok := data["unrealized"].(*decimal.Decimal); ok {
		balance.Unrealized = value
	}

	return balance
}


/*
Request is empty: balance arrives on venue push, not a poll subscription.
*/
func (balance *Balance) Request() map[string]any {
	return map[string]any{}
}

/*
Cash returns quote currency for paper tier sizing and ledger checks.
*/
func (balance *Balance) Cash(quote string) *decimal.Decimal {
	for _, asset := range balance.Assets {
		if asset.Symbol == quote {
			return &asset.Capital
		}
	}

	return nil
}

/*
Reading is the hub wallet shape: cash, unrealized, equity as float64.
*/
func (balance *Balance) Reading() map[string]float64 {
	out := map[string]float64{
		"cash":       0,
		"unrealized": 0,
		"equity":     0,
	}

	if balance == nil {
		return out
	}

	if cash := balance.Cash(balance.Quote); cash != nil {
		out["cash"], _ = cash.Float64()
	}

	if balance.Unrealized != nil {
		out["unrealized"], _ = balance.Unrealized.Float64()
	}

	if balance.Equity != nil {
		out["equity"], _ = balance.Equity.Float64()
	}

	return out
}

/*
ToMarket copies this book into other when other is a *Balance.
*/
func (balance *Balance) ToMarket(other MarketType) MarketType {
	dst, ok := other.(*Balance)

	if !ok || dst == nil || balance == nil {
		return other
	}

	dst.Quote = balance.Quote
	dst.Assets = append([]*Asset(nil), balance.Assets...)
	dst.Equity = balance.Equity
	dst.Unrealized = balance.Unrealized

	return dst
}

/*
FromMarket copies other into this book when other is a *Balance.
*/
func (balance *Balance) FromMarket(other MarketType) MarketType {
	src, ok := other.(*Balance)

	if !ok || src == nil || balance == nil {
		return balance
	}

	return src.ToMarket(balance)
}

/*
MarshalJSON serializes the balance book into a JSON-encoded byte slice.
*/
func (balance *Balance) MarshalJSON() ([]byte, error) {
	type alias Balance
	return json.Marshal((*alias)(balance))
}

/*
UnmarshalJSON deserializes a JSON-encoded byte slice into a balance book.
Replaces the book from JSON.
*/
func (balance *Balance) UnmarshalJSON(data []byte) error {
	type alias Balance
	var decoded alias

	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	*balance = Balance(decoded)


	return nil
}

package market

import (
	"encoding/json"
	"sync"

	"github.com/shopspring/decimal"
)

/*
Balance is spendable cash plus account equity. Cash comes from Balance REST;
equity and unrealized come from TradeBalance REST.
*/
type Balance struct {
	mu         sync.RWMutex
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
	if balance == nil {
		return nil
	}

	balance.mu.RLock()
	defer balance.mu.RUnlock()

	for _, asset := range balance.Assets {
		if asset.Symbol == quote {
			copied := asset.Capital
			return &copied
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

	balance.mu.RLock()
	defer balance.mu.RUnlock()

	for _, asset := range balance.Assets {
		if asset.Symbol == balance.Quote {
			out["cash"], _ = asset.Capital.Float64()
			break
		}
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
ReplaceAssets swaps the spendable asset list without wiping equity/unrealized.
*/
func (balance *Balance) ReplaceAssets(quote string, assets []*Asset) {
	if balance == nil {
		return
	}

	balance.mu.Lock()
	defer balance.mu.Unlock()

	if quote != "" {
		balance.Quote = quote
	}
	balance.Assets = append([]*Asset(nil), assets...)
}

/*
SetUnrealized stores mark PnL under Hello without racing Watch ingest.
*/
func (balance *Balance) SetUnrealized(value *decimal.Decimal) {
	if balance == nil {
		return
	}

	balance.mu.Lock()
	defer balance.mu.Unlock()
	balance.Unrealized = value
}

/*
SetEquity stores TradeBalance equity under ingest without racing Hello.
*/
func (balance *Balance) SetEquity(value *decimal.Decimal) {
	if balance == nil {
		return
	}

	balance.mu.Lock()
	defer balance.mu.Unlock()
	balance.Equity = value
}

/*
ToMarket copies this book into other when other is a *Balance.
*/
func (balance *Balance) ToMarket(other MarketType) MarketType {
	dst, ok := other.(*Balance)

	if !ok || dst == nil || balance == nil {
		return other
	}

	balance.mu.RLock()
	quote := balance.Quote
	assets := append([]*Asset(nil), balance.Assets...)
	equity := balance.Equity
	unrealized := balance.Unrealized
	balance.mu.RUnlock()

	dst.mu.Lock()
	dst.Quote = quote
	dst.Assets = assets
	dst.Equity = equity
	dst.Unrealized = unrealized
	dst.mu.Unlock()

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
	type alias struct {
		Quote      string           `json:"quote"`
		Assets     []*Asset         `json:"assets"`
		Equity     *decimal.Decimal `json:"equity,omitempty"`
		Unrealized *decimal.Decimal `json:"unrealized,omitempty"`
	}

	if balance == nil {
		return json.Marshal(alias{})
	}

	balance.mu.RLock()
	defer balance.mu.RUnlock()

	return json.Marshal(alias{
		Quote:      balance.Quote,
		Assets:     balance.Assets,
		Equity:     balance.Equity,
		Unrealized: balance.Unrealized,
	})
}

/*
UnmarshalJSON deserializes a JSON-encoded byte slice into a balance book.
Replaces the book from JSON.
*/
func (balance *Balance) UnmarshalJSON(data []byte) error {
	type alias struct {
		Quote      string           `json:"quote"`
		Assets     []*Asset         `json:"assets"`
		Equity     *decimal.Decimal `json:"equity,omitempty"`
		Unrealized *decimal.Decimal `json:"unrealized,omitempty"`
	}

	var decoded alias

	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	balance.mu.Lock()
	defer balance.mu.Unlock()

	balance.Quote = decoded.Quote
	balance.Assets = decoded.Assets
	balance.Equity = decoded.Equity
	balance.Unrealized = decoded.Unrealized

	return nil
}

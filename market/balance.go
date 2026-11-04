package market

import (
	"encoding/json"

	"github.com/shopspring/decimal"
)

/*
Balance is the book wallet: quote cash and per-asset capital / pnl / equity.
Venue polling belongs behind Feed; this type only holds and serves state.
*/
type Balance struct {
	Quote    string   `json:"quote"`
	Assets   []*Asset `json:"assets"`
	onChange func(MarketType)
}

/*
NewBalance creates a new balance book with the given quote currency.
*/
func NewBalance(data map[string]any) *Balance {
	return &Balance{
		Quote:  data["quote"].(string),
		Assets: data["assets"].([]*Asset),
	}
}

func (balance *Balance) OnChange(fn func(MarketType)) {
	balance.onChange = fn
}

func (balance *Balance) Request() map[string]any {
	return map[string]any{}
}

func (balance *Balance) Cash(quote string) *decimal.Decimal {
	for _, asset := range balance.Assets {
		if asset.Symbol == quote {
			return &asset.Capital
		}
	}

	return nil
}

/*
ToMarket copies this book into other when other is a *Balance.
*/
func (balance *Balance) ToMarket(other MarketType) MarketType {
	dst, ok := other.(*Balance)

	if !ok || dst == nil || balance == nil {
		return other
	}

	onChange := dst.onChange
	dst.Quote = balance.Quote
	dst.Assets = append([]*Asset(nil), balance.Assets...)
	dst.onChange = onChange

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
Preserves OnChange and notifies after a successful replace.
*/
func (balance *Balance) UnmarshalJSON(data []byte) error {
	type alias Balance
	var decoded alias

	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	onChange := balance.onChange
	*balance = Balance(decoded)
	balance.onChange = onChange

	if balance.onChange != nil {
		balance.onChange(balance)
	}

	return nil
}

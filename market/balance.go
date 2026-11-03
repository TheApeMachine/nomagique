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
	onChange func(*Balance)
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

func (balance *Balance) Cash(quote string) *decimal.Decimal {
	for _, asset := range balance.Assets {
		if asset.Symbol == quote {
			return &asset.Capital
		}
	}

	return nil
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

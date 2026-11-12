package market

import "github.com/shopspring/decimal"

/*
Asset is one leg of Balance book state desk tiers size from before Enter spends.
*/
type Asset struct {
	updater func(asset *Asset)
	Symbol  string          `json:"symbol"`
	Capital decimal.Decimal `json:"capital"`
	PnL     decimal.Decimal `json:"pnl"`
	Equity  decimal.Decimal `json:"equity"`
}

/*
NewAsset wires the balance updater callback so venue fills refresh capital in place.
*/
func NewAsset(
	updater func(asset *Asset),
	symbol string,
	capital decimal.Decimal,
	pnl decimal.Decimal,
	equity decimal.Decimal,
) *Asset {
	return &Asset{
		updater: updater,
		Symbol:  symbol,
		Capital: capital,
		PnL:     pnl,
		Equity:  equity,
	}
}
